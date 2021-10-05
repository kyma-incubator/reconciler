package pod

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"k8s.io/client-go/kubernetes"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	i.log.Infof("Doing rollout for %s %s %s", object.Kind, object.Namespace, object.Name)
	if !i.debug {
		err := retry.Do(func() error {
			err := doRollout(object, i.kubeClient)
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
