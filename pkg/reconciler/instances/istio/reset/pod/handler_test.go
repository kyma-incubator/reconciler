package pod

import (
	"testing"
	"time"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"golang.org/x/sync/errgroup"

	"github.com/stretchr/testify/require"
	v1apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_NoActionHandler_ExecuteAndWaitFor(t *testing.T) {
	t.Run("should execute the NoActionHandler successfully", func(t *testing.T) {
		// given
		customObject := fixCustomObject()
		handler := NoActionHandler{}
		g := new(errgroup.Group)

		// when
		g.Go(func() error {
			err := handler.ExecuteAndWaitFor(*customObject)
			if err != nil {
				return err
			}
			return nil
		})

		// then
		err := g.Wait()
		require.NoError(t, err)
	})
}
func Test_DeleteObjectHandler_ExecuteAndWaitFor(t *testing.T) {
	t.Run("should execute the DeleteObjectHandler successfully", func(t *testing.T) {
		// given
		customObject := fixCustomObject()
		handler := DeleteObjectHandler{handlerCfg{log: log.NewLogger(true), debug: true}}
		g := new(errgroup.Group)

		// when
		g.Go(func() error {
			err := handler.ExecuteAndWaitFor(*customObject)
			if err != nil {
				return err
			}
			return nil
		})

		// then
		err := g.Wait()
		require.NoError(t, err)
	})
}
func Test_RolloutHandler_ExecuteAndWaitFor(t *testing.T) {
	t.Run("should execute the RolloutHandler successfully", func(t *testing.T) {
		// given
		pod := fixCustomObject()
		handler := RolloutHandler{handlerCfg{log: log.NewLogger(true), debug: true}}
		g := new(errgroup.Group)

		// when
		g.Go(func() error {
			err := handler.ExecuteAndWaitFor(*pod)
			if err != nil {
				return err
			}
			return nil
		})

		// then
		err := g.Wait()
		require.NoError(t, err)
	})
}
func Test_RolloutHandler_WaitFor_ReplicaSet(t *testing.T) {
	t.Run("should execute the RolloutHandler successfully on ReplicaSet", func(t *testing.T) {
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
		err := handler.ExecuteAndWaitFor(*pod)
		require.NoError(t, err)

		// when
		err = handler.WaitForResources(*pod)

		// then
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			return isReplicaSetReady(&replicaSetWithOwnerReferences)
		}, time.Second, 10*time.Millisecond)
	})
}
func Test_RolloutHandler_WaitFor_DaemonSet(t *testing.T) {
	t.Run("should execute RolloutHandler successfully on DaemonSet", func(t *testing.T) {
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
		err := handler.ExecuteAndWaitFor(*pod)
		require.NoError(t, err)

		// when
		err = handler.WaitForResources(*pod)

		// then
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			return isDaemonSetReady(&daemonSetWithOwnerReferences)
		}, time.Second, 10*time.Millisecond)
	})
}
func Test_RolloutHandler_WaitFor_StatefulSet(t *testing.T) {
	t.Run("should execute RolloutHandler successfully on StatefulSet", func(t *testing.T) {
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
		err := handler.ExecuteAndWaitFor(*pod)
		require.NoError(t, err)

		// when
		err = handler.WaitForResources(*pod)

		// then
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			return isStatefulSetReady(statefulSetWithOwnerReferences)
		}, time.Second, 10*time.Millisecond)
	})
}
func Test_RolloutHandler_WaitFor_Deployment(t *testing.T) {
	t.Run("should execute RolloutHandler successfully on Deployment", func(t *testing.T) {
		// given
		fixWaitOpts := WaitOptions{
			Interval: 1 * time.Second,
			Timeout:  1 * time.Minute,
		}
		pod := &CustomObject{
			Name:      "test-deploy",
			Namespace: "testns",
			Kind:      "Deployment",
		}
		replicas := int32(2)
		deployment := &v1apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deploy",
				Namespace: "testns",
			},
			Status: v1apps.DeploymentStatus{
				ReadyReplicas: replicas,
			},
			Spec: v1apps.DeploymentSpec{
				Replicas: &replicas,
			},
		}
		replicaSets := []*v1apps.ReplicaSet{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-deploy-1",
					Namespace:       "testns",
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(deployment, deployment.GroupVersionKind())},
				},
				Status: v1apps.ReplicaSetStatus{
					ReadyReplicas: replicas,
				},
			},
		}

		var objects []runtime.Object
		objects = append(objects, deployment)
		for _, rs := range replicaSets {
			objects = append(objects, rs)
		}

		kubeClient := fake.NewSimpleClientset(objects...)
		handler := RolloutHandler{handlerCfg{kubeClient: kubeClient, log: log.NewLogger(true), debug: true, waitOpts: fixWaitOpts}}
		err := handler.ExecuteAndWaitFor(*pod)
		require.NoError(t, err)

		// when
		err = handler.WaitForResources(*pod)

		// then
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			return isDeploymentReady(deployment, kubeClient)
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
