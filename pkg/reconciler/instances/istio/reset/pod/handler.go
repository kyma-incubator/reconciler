package pod

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	tracker "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

//go:generate mockery --name=Handler --outpkg=mocks --case=underscore
// Handler executes actions on the Kubernetes objects.
type Handler interface {
	// Execute action on the Kubernetes object with regards of the type of handler.
	// Returns error if action was unsuccessful or wait timeout was reached.
	ExecuteAndWaitFor(context.Context, CustomObject) error
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

// NoActionHandler that logs information about the pod and does not
// perform any action on the cluster.
type NoActionHandler struct {
	handlerCfg
}

func (i *NoActionHandler) ExecuteAndWaitFor(context context.Context, object CustomObject) error {
	if i.debug {
		i.log.Debugf("Not doing any action for: %s/%s/%s", object.Kind, object.Namespace, object.Name)
	}

	return nil
}

type DeleteObjectHandler struct {
	handlerCfg
}

func (i *DeleteObjectHandler) ExecuteAndWaitFor(context context.Context, object CustomObject) error {
	i.log.Debugf("Deleting pod %s/%s", object.Namespace, object.Name)
	if i.debug {
		return nil
	}

	err := retry.Do(func() error {
		err := i.kubeClient.CoreV1().Pods(object.Namespace).Delete(context, object.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		i.log.Debugf("Deleted pod %s/%s", object.Namespace, object.Name)
		return nil
	}, i.retryOpts...)

	if err != nil {
		return err
	}

	i.log.Debugf("Not waiting for: %s/%s", object.Namespace, object.Name)

	return nil
}

// RolloutHandler that restarts objects
type RolloutHandler struct {
	handlerCfg
}

func (i *RolloutHandler) ExecuteAndWaitFor(context context.Context, object CustomObject) error {
	i.log.Debugf("Doing rollout for %s/%s/%s", object.Kind, object.Namespace, object.Name)
	if i.debug {
		return nil
	}

	err := retry.Do(func() error {
		err := doRollout(context, object, i.kubeClient)
		if err != nil {
			return err
		}

		i.log.Debugf("Rolled out %s/%s/%s", object.Kind, object.Namespace, object.Name)
		return nil
	}, i.retryOpts...)

	if err != nil {
		return err
	}

	return i.WaitForResources(context, object)
}

func (i *RolloutHandler) WaitForResources(context context.Context, object CustomObject) error {
	i.log.Debugf("Waiting for %s/%s/%s to be ready", object.Kind, object.Namespace, object.Name)
	pt, err := tracker.NewProgressTracker(i.kubeClient, i.log, tracker.Config{Interval: i.waitOpts.Interval, Timeout: i.waitOpts.Timeout})
	if err != nil {
		return errors.Wrap(err, "Failed to setup the tracker")
	}

	watchable, err := tracker.NewWatchableResource(object.Kind)
	if err == nil {
		i.log.Debugf("Register watchable %s '%s' in namespace '%s'", object.Kind, object.Name, object.Namespace)
		pt.AddResource(watchable, object.Namespace, object.Name)
	} else {
		return errors.Wrap(err, "Failed to register watchable resources")
	}

	err = pt.Watch(context, tracker.ReadyState)
	if err != nil {
		return errors.Wrapf(err, "Failed to wait for %s/%s/%s to be rolled out", object.Kind, object.Namespace, object.Name)
	}
	i.log.Debugf("%s/%s/%s is ready", object.Kind, object.Namespace, object.Name)

	return nil
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

func doRollout(context context.Context, customObject CustomObject, kubeClient kubernetes.Interface) (err error) {
	data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().String())

	switch customObject.Kind {
	case "DaemonSet":
		_, err = kubeClient.AppsV1().DaemonSets(customObject.Namespace).Patch(context, customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	case "Deployment":
		_, err = kubeClient.AppsV1().Deployments(customObject.Namespace).Patch(context, customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	case "ReplicaSet":
		_, err = kubeClient.AppsV1().ReplicaSets(customObject.Namespace).Patch(context, customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	case "StatefulSet":
		_, err = kubeClient.AppsV1().StatefulSets(customObject.Namespace).Patch(context, customObject.Name, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	default:
		err = fmt.Errorf("kind %s not found", customObject.Kind)
	}

	if err != nil {
		return err
	}

	return
}
