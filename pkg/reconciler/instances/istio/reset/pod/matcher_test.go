package pod

import (
	"reflect"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/logger"

	"github.com/stretchr/testify/require"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_ParentKindMatcher_Match(t *testing.T) {
	log := logger.NewLogger(true)
	debug := true
	fixRetryOpts := []retry.Option{
		retry.Delay(1 * time.Second),
		retry.Attempts(1),
		retry.DelayType(retry.FixedDelay),
	}

	fixWaitOpts := WaitOptions{
		Timeout:  5 * time.Minute,
		Interval: 5 * time.Second,
	}

	t.Run("should return NoActionHandler when pod has no owner", func(t *testing.T) {
		// given
		podList := fixPodListWithParentKind("")
		kubeClient := fake.NewSimpleClientset()
		matcher := ParentKindMatcher{}

		// when
		handlersMap := matcher.GetHandlersMap(kubeClient, fixRetryOpts, *podList, log, debug, fixWaitOpts)

		// then
		require.NotNil(t, handlersMap)
		require.Len(t, handlersMap, 1)
		for k := range handlersMap {
			require.Contains(t, reflect.TypeOf(k).String(), "NoActionHandler")
		}
	})

	t.Run("should return RolloutHandler when pod has unknown owner", func(t *testing.T) {
		// given
		podList := fixPodListWithParentKind("unknown")
		kubeClient := fake.NewSimpleClientset()
		matcher := ParentKindMatcher{}

		// when
		handlersMap := matcher.GetHandlersMap(kubeClient, fixRetryOpts, *podList, log, debug, fixWaitOpts)

		// then
		require.NotNil(t, handlersMap)
		require.Len(t, handlersMap, 1)
		for k := range handlersMap {
			require.Contains(t, reflect.TypeOf(k).String(), "RolloutHandler")
		}
	})

	t.Run("should return RolloutHandler when pod has an owner of ReplicaSet kind and owner has a parent", func(t *testing.T) {
		// given
		podList := fixPodListWithParentKind("ReplicaSet")
		replicaSetWithOwnerReferences := v1apps.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Name: "name", Kind: "kind"},
				},
				Name:      "ownername",
				Namespace: "namespace",
			}}
		kubeClient := fake.NewSimpleClientset(&replicaSetWithOwnerReferences)
		matcher := ParentKindMatcher{}

		// when
		handlersMap := matcher.GetHandlersMap(kubeClient, fixRetryOpts, *podList, log, debug, fixWaitOpts)

		// then
		require.NotNil(t, handlersMap)
		require.Len(t, handlersMap, 1)
		for k := range handlersMap {
			require.Contains(t, reflect.TypeOf(k).String(), "RolloutHandler")
		}
	})

	t.Run("should return DeleteObjectHandler when pod has an owner of ReplicaSet kind and owner does not have a parent", func(t *testing.T) {
		// given
		podList := fixPodListWithParentKind("ReplicaSet")
		replicaSetWithoutOwnerReferences := v1apps.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{},
				Name:            "ownername",
				Namespace:       "namespace",
			}}
		kubeClient := fake.NewSimpleClientset(&replicaSetWithoutOwnerReferences)
		matcher := ParentKindMatcher{}

		// when
		handlersMap := matcher.GetHandlersMap(kubeClient, fixRetryOpts, *podList, log, debug, fixWaitOpts)

		// then
		require.NotNil(t, handlersMap)
		require.Len(t, handlersMap, 1)
		for k := range handlersMap {
			require.Contains(t, reflect.TypeOf(k).String(), "DeleteObjectHandler")
		}
	})

	t.Run("should return DeleteObjectHandler when pod has an owner of ReplicationController kind", func(t *testing.T) {
		// given
		podList := fixPodListWithParentKind("ReplicationController")
		kubeClient := fake.NewSimpleClientset()
		matcher := ParentKindMatcher{}

		// when
		handlersMap := matcher.GetHandlersMap(kubeClient, fixRetryOpts, *podList, log, debug, fixWaitOpts)

		// then
		require.NotNil(t, handlersMap)
		require.Len(t, handlersMap, 1)
		for k := range handlersMap {
			require.Contains(t, reflect.TypeOf(k).String(), "DeleteObjectHandler")
		}
	})

	t.Run("should return one RolloutHandler when two pods have the same owner of ReplicaSet kind and owner has a parent", func(t *testing.T) {
		// given
		podList := fixPodListWithParentKind("ReplicaSet")
		podList.Items = append(podList.Items, fixPodListWithParentKind("ReplicaSet").Items[0])
		replicaSetWithOwnerReferences := v1apps.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Name: "name", Kind: "kind"},
				},
				Name:      "ownername",
				Namespace: "namespace",
			}}
		kubeClient := fake.NewSimpleClientset(&replicaSetWithOwnerReferences)
		matcher := ParentKindMatcher{}

		// when
		handlersMap := matcher.GetHandlersMap(kubeClient, fixRetryOpts, *podList, log, debug, fixWaitOpts)

		// then
		require.Len(t, podList.Items, 2)
		require.NotNil(t, handlersMap)
		require.Len(t, handlersMap, 1)
		for k := range handlersMap {
			require.Contains(t, reflect.TypeOf(k).String(), "RolloutHandler")
		}
	})

	t.Run("should return one RolloutHandler with two replicaSets when two pods have different owner of ReplicaSet kind", func(t *testing.T) {
		// given
		podList := fixPodListWithParentKind("ReplicaSet")
		podList.Items[0].ObjectMeta.OwnerReferences[0].Name = "ownername2"
		podList.Items = append(podList.Items, fixPodListWithParentKind("ReplicaSet").Items[0])

		replicaSetWithOwnerReferences := v1apps.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Name: "name", Kind: "kind"},
				},
				Name:      "ownername",
				Namespace: "namespace",
			}}
		replicaSetWithOwnerReferences2 := v1apps.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					// parent of replicaset pod has to be different here
					{Name: "name2", Kind: "kind"},
				},
				Name:      "ownername2",
				Namespace: "namespace",
			}}

		kubeClient := fake.NewSimpleClientset(&replicaSetWithOwnerReferences, &replicaSetWithOwnerReferences2)
		matcher := ParentKindMatcher{}

		// when
		handlersMap := matcher.GetHandlersMap(kubeClient, fixRetryOpts, *podList, log, debug, fixWaitOpts)

		// then
		require.Len(t, podList.Items, 2)
		require.NotNil(t, handlersMap)
		require.Len(t, handlersMap, 1)
		for k, v := range handlersMap {
			require.Contains(t, reflect.TypeOf(k).String(), "RolloutHandler")
			require.Len(t, v, 2)
		}
	})
}

func fixPodListWithParentKind(kind string) *v1.PodList {
	return &v1.PodList{
		Items: []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: kind,
							Name: "ownername",
						},
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
			},
		},
	}
}
