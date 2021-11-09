package pod

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kctlutil "k8s.io/kubectl/pkg/util/deployment"
)

type GetSyncWG func() *sync.WaitGroup

//go:generate mockery --name=Handler --outpkg=mocks --case=underscore
// Handler executes actions on the Kubernetes cluster
type Handler interface {
	Execute(CustomObject, GetSyncWG)
	WaitForResources(CustomObject, GetSyncWG) error
}

type WaitOptions struct {
	Interval time.Duration
	Timeout  time.Duration
}

type handlerCfg struct {
	kubeClient kubernetes.Interface
	retryOpts  []retry.Option
	log        *zap.SugaredLogger
	debug      bool
	waitOpts   WaitOptions
}

// NoActionHandler that logs information about the pod and does not
// perform any action on the cluster.
type NoActionHandler struct {
	handlerCfg
}

func (i *NoActionHandler) Execute(object CustomObject, wg GetSyncWG) {
	defer wg().Done()

	if i.debug {
		i.log.Infof("Not doing any action for: %s/%s/%s", object.Kind, object.Namespace, object.Name)
	}
}

func (i *NoActionHandler) WaitForResources(object CustomObject, wg GetSyncWG) error {
	defer wg().Done()

	if i.debug {
		i.log.Infof("Not waiting for: %s/%s/%s", object.Kind, object.Namespace, object.Name)
	}

	return nil
}

type DeleteObjectHandler struct {
	handlerCfg
}

func (i *DeleteObjectHandler) Execute(object CustomObject, wg GetSyncWG) {
	defer wg().Done()

	i.log.Infof("Deleting pod %s/%s", object.Namespace, object.Name)
	if !i.debug {
		err := retry.Do(func() error {
			err := i.kubeClient.CoreV1().Pods(object.Namespace).Delete(context.Background(), object.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}

			i.log.Infof("Deleted pod %s/%s", object.Namespace, object.Name)
			return nil
		}, i.retryOpts...)

		if err != nil {
			i.log.Error(err)
		}
	}
}

func (i *DeleteObjectHandler) WaitForResources(object CustomObject, wg GetSyncWG) error {
	defer wg().Done()

	if i.debug {
		i.log.Infof("Not waiting for: %s/%s/%s", object.Kind, object.Namespace, object.Name)
	}

	return nil
}

// RolloutHandler that restarts objects
type RolloutHandler struct {
	handlerCfg
}

func (i *RolloutHandler) Execute(object CustomObject, wg GetSyncWG) {
	defer wg().Done()

	i.log.Infof("Doing rollout for %s/%s/%s", object.Kind, object.Namespace, object.Name)
	if !i.debug {
		err := retry.Do(func() error {
			err := doRollout(object, i.kubeClient)
			if err != nil {
				return err
			}

			i.log.Infof("Rolled out %s/%s/%s", object.Kind, object.Namespace, object.Name)
			return nil
		}, i.retryOpts...)

		if err != nil {
			i.log.Error(err)
		}
	}
}

func (i *RolloutHandler) WaitForResources(object CustomObject, wg GetSyncWG) (err error) {
	defer wg().Done()

	i.log.Infof("Waiting for %s/%s/%s to be ready", object.Kind, object.Namespace, object.Name)
	switch object.Kind {
	case "DaemonSet":
		err = wait.Poll(i.waitOpts.Interval, i.waitOpts.Timeout, func() (done bool, err error) {
			ds, err := i.kubeClient.AppsV1().DaemonSets(object.Namespace).Get(context.Background(), object.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready := isDaemonSetReady(ds)
			return ready, nil
		})
	case "Deployment":
		err = wait.Poll(i.waitOpts.Interval, i.waitOpts.Timeout, func() (done bool, err error) {
			dep, err := i.kubeClient.AppsV1().Deployments(object.Namespace).Get(context.Background(), object.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready := isDeploymentReady(dep, i.kubeClient)
			return ready, nil
		})
	case "ReplicaSet":
		err = wait.Poll(i.waitOpts.Interval, i.waitOpts.Timeout, func() (done bool, err error) {
			rs, err := i.kubeClient.AppsV1().ReplicaSets(object.Namespace).Get(context.Background(), object.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready := isReplicaSetReady(rs)
			return ready, nil
		})
	case "StatefulSet":
		err = wait.Poll(i.waitOpts.Interval, i.waitOpts.Timeout, func() (done bool, err error) {
			sts, err := i.kubeClient.AppsV1().StatefulSets(object.Namespace).Get(context.Background(), object.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready := isStatefulSetReady(sts)
			return ready, nil
		})
	default:
		err = fmt.Errorf("kind %s not found", object.Kind)
	}

	if err != nil {
		return err
	}

	return nil
}

type parentObject struct {
	Name string
	Kind string
}

// CustomObject contains all necessary fields to do rollout.
type CustomObject struct {
	Name      string
	Namespace string
	Kind      string
}

func getParentObjectFromOwnerReferences(ownerReferences []metav1.OwnerReference) parentObject {
	if len(ownerReferences) == 0 {
		return parentObject{}
	}

	ownerReference := ownerReferences[0].DeepCopy()
	return parentObject{
		Name: ownerReference.Name,
		Kind: ownerReference.Kind,
	}
}

func doRollout(customObject CustomObject, kubeClient kubernetes.Interface) (err error) {
	data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().String())

	switch customObject.Kind {
	case "DaemonSet":
		_, err = kubeClient.AppsV1().DaemonSets(customObject.Namespace).Patch(context.Background(), customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	case "Deployment":
		_, err = kubeClient.AppsV1().Deployments(customObject.Namespace).Patch(context.Background(), customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	case "ReplicaSet":
		_, err = kubeClient.AppsV1().ReplicaSets(customObject.Namespace).Patch(context.Background(), customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	case "StatefulSet":
		_, err = kubeClient.AppsV1().StatefulSets(customObject.Namespace).Patch(context.Background(), customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	default:
		err = fmt.Errorf("kind %s not found", customObject.Kind)
	}

	if err != nil {
		return err
	}

	return
}

func isDeploymentReady(deployment *v1.Deployment, kubeClient kubernetes.Interface) bool {
	if deployment.DeletionTimestamp != nil {
		return false
	}
	_, _, newReplicaSet, err := kctlutil.GetAllReplicaSets(deployment, kubeClient.AppsV1())
	if err != nil || newReplicaSet == nil {
		return false
	}
	if newReplicaSet.Status.ReadyReplicas < *deployment.Spec.Replicas {
		return false
	}

	return true
}

func isStatefulSetReady(sts *v1.StatefulSet) bool {
	if sts.DeletionTimestamp != nil {
		return false
	}

	if sts.Generation <= sts.Status.ObservedGeneration {
		if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
			return false
		}
		if sts.Status.ReadyReplicas != *sts.Spec.Replicas {
			return false
		}
	}

	return true
}

func isDaemonSetReady(ds *v1.DaemonSet) bool {
	if ds.DeletionTimestamp != nil {
		return false
	}

	if ds.Generation <= ds.Status.ObservedGeneration {
		if ds.Status.UpdatedNumberScheduled < ds.Status.DesiredNumberScheduled {
			return false
		}
		if ds.Status.NumberAvailable < ds.Status.DesiredNumberScheduled {
			return false
		}
	}

	return true
}

func isReplicaSetReady(rs *v1.ReplicaSet) bool {
	if rs.DeletionTimestamp != nil {
		return false
	}
	if rs.Status.Replicas != rs.Status.AvailableReplicas &&
		rs.Status.Replicas != rs.Status.ReadyReplicas {
		return false
	}

	return true
}
