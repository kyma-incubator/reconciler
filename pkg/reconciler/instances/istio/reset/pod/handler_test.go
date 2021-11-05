package pod

import (
	"testing"
	"time"

	log "github.com/kyma-incubator/reconciler/pkg/logger"

	"github.com/stretchr/testify/require"
	v1apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_NoActionHandler_Execute(t *testing.T) {
	t.Run("should execute the NoActionHandler successfully", func(t *testing.T) {
		// given
		customObject := fixCustomObject()
		handler := NoActionHandler{}

		// when
		err := handler.ExecuteAndWaitFor(*customObject)
		require.NoError(t, err)

		// then
		require.Eventually(t, func() bool {
			return true
		}, time.Second, 10*time.Millisecond)
	})
}

func Test_DeleteObjectHandler_Execute(t *testing.T) {
	t.Run("should execute the DeleteObjectHandler successfully", func(t *testing.T) {
		// given
		customObject := fixCustomObject()
		handler := DeleteObjectHandler{handlerCfg{log: log.NewLogger(true), debug: true}}

		// when
		err := handler.ExecuteAndWaitFor(*customObject)
		require.NoError(t, err)

		// then
		require.Eventually(t, func() bool {
			return true
		}, time.Second, 10*time.Millisecond)
	})
}

func Test_RolloutHandler_Execute(t *testing.T) {
	t.Run("should execute the RolloutHandler successfully", func(t *testing.T) {
		// given
		pod := fixCustomObject()
		handler := RolloutHandler{handlerCfg{log: log.NewLogger(true), debug: true}}

		// when
		err := handler.ExecuteAndWaitFor(*pod)
		require.NoError(t, err)

		// then
		require.Eventually(t, func() bool {
			return true
		}, time.Second, 10*time.Millisecond)
	})
}
func Test_NoActionHandler_WaitForResources(t *testing.T) {
	t.Run("should run the WaitForResources with NoActionHandler successfully", func(t *testing.T) {
		// given
		customObject := fixCustomObject()
		handler := NoActionHandler{}

		// when
		err := handler.ExecuteAndWaitFor(*customObject)
		require.NoError(t, err)

		// then
		require.Eventually(t, func() bool {
			return true
		}, time.Second, 10*time.Millisecond)
	})
}

func Test_DeleteObjectHandler_WaitForResources(t *testing.T) {
	t.Run("should run the WaitForResources with DeleteObjectHandler successfully", func(t *testing.T) {
		// given
		customObject := fixCustomObject()
		handler := DeleteObjectHandler{handlerCfg{log: log.NewLogger(true), debug: true}}

		// when
		err := handler.ExecuteAndWaitFor(*customObject)
		require.NoError(t, err)

		// then
		require.Eventually(t, func() bool {
			return true
		}, time.Second, 10*time.Millisecond)
	})
}

func Test_RolloutHandler_WaitForResources_ReplicaSet(t *testing.T) {
	t.Run("should run the WaitForResources with RolloutHandler successfully on ReplicaSet", func(t *testing.T) {
		// given
		fixWaitOpts := WaitOptions{
			Interval: 1 * time.Second,
			Timeout:  1 * time.Minute,
		}
		pod := fixCustomObject()
		replicaSetWithOwnerReferences := v1apps.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Name: "name", Kind: "kind"},
				},
				Name:      "testObject",
				Namespace: "testNamespace",
			}}
		kubeClient := fake.NewSimpleClientset(&replicaSetWithOwnerReferences)
		handler := RolloutHandler{handlerCfg{kubeClient: kubeClient, log: log.NewLogger(true), debug: true, waitOpts: fixWaitOpts}}

		// when
		err := handler.ExecuteAndWaitFor(*pod)
		require.NoError(t, err)

		// then
		require.Eventually(t, func() bool {
			return true
		}, time.Second, 10*time.Millisecond)
	})
}
func Test_RolloutHandler_WaitForResources_DaemonSet(t *testing.T) {
	t.Run("should run the WaitForResources with RolloutHandler successfully on DaemonSet", func(t *testing.T) {
		// given
		fixWaitOpts := WaitOptions{
			Interval: 1 * time.Second,
			Timeout:  1 * time.Minute,
		}
		pod := &CustomObject{
			Name:      "test-ds",
			Namespace: "testns",
			Kind:      "DaemonSet",
		}
		daemonSetWithOwnerReferences := v1apps.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Name: "name", Kind: "kind"},
				},
				Name:      "test-ds",
				Namespace: "testns",
			}}
		kubeClient := fake.NewSimpleClientset(&daemonSetWithOwnerReferences)
		handler := RolloutHandler{handlerCfg{kubeClient: kubeClient, log: log.NewLogger(true), debug: true, waitOpts: fixWaitOpts}}

		// when
		err := handler.ExecuteAndWaitFor(*pod)
		require.NoError(t, err)

		// then
		require.Eventually(t, func() bool {
			return true
		}, time.Second, 10*time.Millisecond)
	})
}
func Test_RolloutHandler_WaitForResources_StatefulSet(t *testing.T) {
	t.Run("should run the WaitForResources with RolloutHandler successfully on StatefulSet", func(t *testing.T) {
		// given
		fixWaitOpts := WaitOptions{
			Interval: 1 * time.Second,
			Timeout:  1 * time.Minute,
		}
		pod := &CustomObject{
			Name:      "test-sts",
			Namespace: "testns",
			Kind:      "StatefulSet",
		}
		replicas := int32(1)
		statefulSetWithOwnerReferences := &v1apps.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-sts",
				Namespace: "testns",
			},
			Spec: v1apps.StatefulSetSpec{
				Replicas: &replicas,
			},
			Status: v1apps.StatefulSetStatus{
				ReadyReplicas: 1,
			},
		}
		kubeClient := fake.NewSimpleClientset(statefulSetWithOwnerReferences)
		handler := RolloutHandler{handlerCfg{kubeClient: kubeClient, log: log.NewLogger(true), debug: true, waitOpts: fixWaitOpts}}

		// when
		err := handler.ExecuteAndWaitFor(*pod)
		require.NoError(t, err)

		// then
		require.Eventually(t, func() bool {
			return true
		}, time.Second, 10*time.Millisecond)
	})
}
func Test_getParentObjectFromOwnerReferences(t *testing.T) {
	t.Run("should return empty parentObject when there is no owner references", func(t *testing.T) {
		// given
		ownerReference := []metav1.OwnerReference{}

		// when
		parentObjectData := getParentObjectFromOwnerReferences(ownerReference)

		// then
		require.NotNil(t, parentObjectData)
		require.Empty(t, parentObjectData)
	})

	t.Run("should return empty parentObject when owner reference is nil", func(t *testing.T) {
		// when
		parentObjectData := getParentObjectFromOwnerReferences(nil)

		// then
		require.NotNil(t, parentObjectData)
		require.Empty(t, parentObjectData)
	})

	t.Run("should return parentObject with data from one owner reference", func(t *testing.T) {
		// given
		ownerReference := []metav1.OwnerReference{{Name: "Name", Kind: "Kind"}}

		// when
		parentObjectData := getParentObjectFromOwnerReferences(ownerReference)

		// then
		require.NotNil(t, parentObjectData)
		require.Equal(t, parentObject{Name: "Name", Kind: "Kind"}, parentObjectData)
	})

	t.Run("should return parentObject with data from the first owner reference when there are two owner references", func(t *testing.T) {
		// given
		ownerReference := []metav1.OwnerReference{{Name: "Name1", Kind: "Kind1"}, {Name: "Name2", Kind: "Kind2"}}

		// when
		parentObjectData := getParentObjectFromOwnerReferences(ownerReference)

		// then
		require.NotNil(t, parentObjectData)
		require.Equal(t, parentObject{Name: "Name1", Kind: "Kind1"}, parentObjectData)
	})
}

func fixCustomObject() *CustomObject {
	return &CustomObject{
		Name:      "testObject",
		Namespace: "testNamespace",
		Kind:      "ReplicaSet",
	}
}
