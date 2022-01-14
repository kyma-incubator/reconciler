package pod

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	tracker "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kctlutil "k8s.io/kubectl/pkg/util/deployment"
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
		i.log.Infof("Not doing any action for: %s/%s/%s", object.Kind, object.Namespace, object.Name)
	}

	return nil
}

type DeleteObjectHandler struct {
	handlerCfg
}

func (i *DeleteObjectHandler) ExecuteAndWaitFor(context context.Context, object CustomObject) error {
	i.log.Infof("Deleting pod %s/%s", object.Namespace, object.Name)
	if i.debug {
		return nil
	}

	err := retry.Do(func() error {
		err := i.kubeClient.CoreV1().Pods(object.Namespace).Delete(context, object.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		i.log.Infof("Deleted pod %s/%s", object.Namespace, object.Name)
		return nil
	}, i.retryOpts...)

	if err != nil {
		return err
	}

	i.log.Infof("Not waiting for: %s/%s", object.Namespace, object.Name)

	return nil
}

// RolloutHandler that restarts objects
type RolloutHandler struct {
	handlerCfg
}

func (i *RolloutHandler) ExecuteAndWaitFor(context context.Context, object CustomObject) error {
	i.log.Infof("Doing rollout for %s/%s/%s", object.Kind, object.Namespace, object.Name)
	if i.debug {
		return nil
	}

	err := retry.Do(func() error {
		err := doRollout(context, object, i.kubeClient)
		if err != nil {
			return err
		}

		i.log.Infof("Rolled out %s/%s/%s", object.Kind, object.Namespace, object.Name)
		return nil
	}, i.retryOpts...)

	if err != nil {
		return err
	}

	return i.WaitForResources(context, object)
}

func (i *RolloutHandler) WaitForResources(context context.Context, object CustomObject) error {
	i.log.Infof("Waiting for %s/%s/%s to be ready", object.Kind, object.Namespace, object.Name)
	switch object.Kind {
	case "DaemonSet":
		err := i.waitForDaemonSet(context, object)
		if err != nil {
			return err
		}
	case "Deployment":
		err := i.waitForDeployment(context, object)
		if err != nil {
			return err
		}
	case "StatefulSet":
		err := i.waitForStatefulSet(context, object)
		if err != nil {
			return err
		}
	case "ReplicaSet":
		err := i.waitForReplicaSet(context, object)
		if err != nil {
			return err
		}
	default:
		i.log.Infof("Not waiting for: %s/%s/%s", object.Kind, object.Namespace, object.Name)
	}

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

func (i *RolloutHandler) waitForDeployment(context context.Context, object CustomObject) error {
	pt, _ := tracker.NewProgressTracker(i.kubeClient, i.log, tracker.Config{Interval: i.waitOpts.Interval, Timeout: i.waitOpts.Timeout})
	watchable, err2 := tracker.NewWatchableResource(object.Kind)
	if err2 == nil {
		i.log.Infof("Register watchable %s '%s' in namespace '%s'", object.Kind, object.Name, object.Namespace)
		pt.AddResource(watchable, object.Namespace, object.Name)
	} else {
		return errors.Wrap(err2, "Failed to register watchable resources")
	}

	err := pt.Watch(context, tracker.ReadyState)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be rolled out")
	}
	i.log.Infof("%s/%s/%s is ready", object.Kind, object.Namespace, object.Name)

	return nil
}

func (i *RolloutHandler) waitForDaemonSet(context context.Context, object CustomObject) error {
	pt, _ := tracker.NewProgressTracker(i.kubeClient, i.log, tracker.Config{Interval: i.waitOpts.Interval, Timeout: i.waitOpts.Timeout})
	watchable, err2 := tracker.NewWatchableResource(object.Kind)
	if err2 == nil {
		i.log.Infof("Register watchable %s '%s' in namespace '%s'", object.Kind, object.Name, object.Namespace)
		pt.AddResource(watchable, object.Namespace, object.Name)
	} else {
		return errors.Wrap(err2, "Failed to register watchable resources")
	}

	err := pt.Watch(context, tracker.ReadyState)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be rolled out")
	}
	i.log.Infof("%s/%s/%s is ready", object.Kind, object.Namespace, object.Name)

	return nil
}

func (i *RolloutHandler) waitForStatefulSet(context context.Context, object CustomObject) error {
	pt, _ := tracker.NewProgressTracker(i.kubeClient, i.log, tracker.Config{Interval: i.waitOpts.Interval, Timeout: i.waitOpts.Timeout})
	watchable, err2 := tracker.NewWatchableResource(object.Kind)
	if err2 == nil {
		i.log.Infof("Register watchable %s '%s' in namespace '%s'", object.Kind, object.Name, object.Namespace)
		pt.AddResource(watchable, object.Namespace, object.Name)
	} else {
		return errors.Wrap(err2, "Failed to register watchable resources")
	}

	err := pt.Watch(context, tracker.ReadyState)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be rolled out")
	}
	i.log.Infof("%s/%s/%s is ready", object.Kind, object.Namespace, object.Name)

	return nil
}

func (i *RolloutHandler) waitForReplicaSet(context context.Context, object CustomObject) error {
	pt, _ := tracker.NewProgressTracker(i.kubeClient, i.log, tracker.Config{Interval: i.waitOpts.Interval, Timeout: i.waitOpts.Timeout})
	watchable, err2 := tracker.NewWatchableResource(object.Kind)
	if err2 == nil {
		i.log.Debugf("Register watchable %s '%s' in namespace '%s'", object.Kind, object.Name, object.Namespace)
		pt.AddResource(watchable, object.Namespace, object.Name)
	} else {
		return errors.Wrap(err2, "Failed to register watchable resources")
	}

	err := pt.Watch(context, tracker.ReadyState)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be rolled out")
	}
	i.log.Infof("%s/%s/%s is ready", object.Kind, object.Namespace, object.Name)

	return nil
}
