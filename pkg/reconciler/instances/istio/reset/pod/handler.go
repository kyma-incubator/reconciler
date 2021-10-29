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
}

type handlerCfg struct {
	kubeClient kubernetes.Interface
	retryOpts  []retry.Option
	log        *zap.SugaredLogger
	debug      bool
}

// NoActionHandler that logs information about the pod and does not
// perform any action on the cluster.
type NoActionHandler struct {
	handlerCfg
}

func (i *NoActionHandler) Execute(object CustomObject, wg GetSyncWG) {
	defer wg().Done()

	if i.debug {
		i.log.Infof("Not doing any action for: %s %s %s", object.Kind, object.Namespace, object.Name)
	}
}

type DeleteObjectHandler struct {
	handlerCfg
}

func (i *DeleteObjectHandler) Execute(object CustomObject, wg GetSyncWG) {
	defer wg().Done()

	i.log.Infof("Deleting pod %s %s", object.Name, object.Namespace)
	if !i.debug {
		err := retry.Do(func() error {
			err := i.kubeClient.CoreV1().Pods(object.Namespace).Delete(context.Background(), object.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}

			i.log.Infof("Deleted pod: %s", object.Name)
			return nil
		}, i.retryOpts...)

		if err != nil {
			i.log.Error(err)
		}
	}
}

// RolloutHandler that restarts objects
type RolloutHandler struct {
	handlerCfg
}

func (i *RolloutHandler) Execute(object CustomObject, wg GetSyncWG) {
	defer wg().Done()

	i.log.Infof("Doing rollout and waiting for %s %s %s", object.Kind, object.Namespace, object.Name)
	if !i.debug {
		err := retry.Do(func() error {
			err := doRolloutAndWait(object, i.kubeClient)
			if err != nil {
				return err
			}

			i.log.Infof("Rolled out deployment: %s", object.Name)
			return nil
		}, i.retryOpts...)

		if err != nil {
			i.log.Error(err)
		}
	}
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

func doRolloutAndWait(customObject CustomObject, kubeClient kubernetes.Interface) (err error) {
	data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().String())

	switch customObject.Kind {
	case "DaemonSet":
		_, err = kubeClient.AppsV1().DaemonSets(customObject.Namespace).Patch(context.Background(), customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return err
		}
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
			ds, err := kubeClient.AppsV1().DaemonSets(customObject.Namespace).Get(context.Background(), customObject.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready := isDaemonSetReady(ds, kubeClient)
			return ready, nil
		})
		if err != nil {
			return err
		}
	case "Deployment":
		_, err = kubeClient.AppsV1().Deployments(customObject.Namespace).Patch(context.Background(), customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return err
		}
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
			dep, err := kubeClient.AppsV1().Deployments(customObject.Namespace).Get(context.Background(), customObject.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready := isDeploymentReady(dep, kubeClient)
			return ready, nil
		})
		if err != nil {
			return err
		}
	case "ReplicaSet":
		_, err = kubeClient.AppsV1().ReplicaSets(customObject.Namespace).Patch(context.Background(), customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return err
		}
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
			rs, err := kubeClient.AppsV1().ReplicaSets(customObject.Namespace).Get(context.Background(), customObject.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready := isReplicaSetReady(rs, kubeClient)
			return ready, nil
		})
		if err != nil {
			return err
		}
	case "StatefulSet":
		_, err = kubeClient.AppsV1().StatefulSets(customObject.Namespace).Patch(context.Background(), customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return err
		}
		err = wait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
			sts, err := kubeClient.AppsV1().StatefulSets(customObject.Namespace).Get(context.Background(), customObject.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ready := isStatefulSetReady(sts, kubeClient)
			return ready, nil
		})
		if err != nil {
			return err
		}
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

func isStatefulSetReady(sts *v1.StatefulSet, kubeClient kubernetes.Interface) bool {
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

func isDaemonSetReady(ds *v1.DaemonSet, kubeClient kubernetes.Interface) bool {

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

func isReplicaSetReady(rs *v1.ReplicaSet, kubeClient kubernetes.Interface) bool {
	if rs.DeletionTimestamp != nil {
		return false
	}
	if rs.Status.Replicas != rs.Status.AvailableReplicas &&
		rs.Status.Replicas != rs.Status.ReadyReplicas {
		return false
	}

	return true
}
