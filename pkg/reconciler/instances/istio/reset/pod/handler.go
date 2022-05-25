package pod

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	tracker "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sRetry "k8s.io/client-go/util/retry"
)

const (
	AnnotationResetWarningKey               = "istio.reconciler.kyma-project.io/proxy-reset-warning"
	AnnotationResetWarningNoOwnerVal        = "pod sidecar could not be updated because OwnerReferences is Job or was not found . Istio might not work. Recreate the pod manually to ensure full compatibility."
	AnnotationResetWarningRolloutTimeoutVal = "pod could not be rolled out by resource owner's controller. Check pod status and resolve the problem so the owner controller can reconcile successfully"
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
		i.log.Debugf("annotating object where we don't perform proxy reset for: %s/%s/%s", object.Kind, object.Namespace, object.Name)
	}
	err := k8sRetry.RetryOnConflict(k8sRetry.DefaultRetry, func() error {
		pod, err := i.kubeClient.CoreV1().Pods(object.Namespace).Get(context, object.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[AnnotationResetWarningKey] = AnnotationResetWarningNoOwnerVal
		_, err = i.kubeClient.CoreV1().Pods(object.Namespace).Update(context, pod, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	return err
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
		i.log.Warnf("unable to rollout %s/%s/%s: %v", object.Kind, object.Namespace, object.Name, err)
		errAnnotate := k8sRetry.RetryOnConflict(k8sRetry.DefaultRetry, func() error {
			client := i.kubeClient
			err := annotateObject(context, client, object, AnnotationResetWarningKey, AnnotationResetWarningRolloutTimeoutVal)
			return err
		})

		if errAnnotate != nil {
			return errAnnotate
		}
		return err
	}

	err = i.WaitForResources(context, object)
	if err != nil {
		errAnnotate := k8sRetry.RetryOnConflict(k8sRetry.DefaultRetry, func() error {
			client := i.kubeClient
			err := annotateObject(context, client, object, AnnotationResetWarningKey, AnnotationResetWarningRolloutTimeoutVal)
			return err
		})

		if errAnnotate != nil {
			return errAnnotate
		}
		return err
	}

	return nil
}

func annotateObject(context context.Context, client kubernetes.Interface, object CustomObject, key string, val string) error {
	switch object.Kind {
	case "Deployment":
		err := annotatePodsFor(context, getDeploymentLabelSelector, client, object, key, val)
		if err != nil {
			return err
		}
	case "DaemonSet":
		err := annotatePodsFor(context, getDaemonSetLabelSelector, client, object, key, val)
		if err != nil {
			return err
		}
	case "ReplicaSet":
		err := annotatePodsFor(context, getReplicaSetLabelSelector, client, object, key, val)
		if err != nil {
			return err
		}

	case "StatefulSet":
		err := annotatePodsFor(context, getStatefulSetLabelSelector, client, object, key, val)
		if err != nil {
			return err
		}
	case "Job":
		err := annotatePodsFor(context, getJobLabelSelector, client, object, key, val)
		if err != nil {
			return err
		}
	default:
		err := fmt.Errorf("kind %s not found", object.Kind)
		return err
	}
	return nil
}

type labelSelectorGetter func(context context.Context, client kubernetes.Interface, object CustomObject) (string, error)

func annotatePodsFor(context context.Context, f labelSelectorGetter, client kubernetes.Interface, object CustomObject, annotationKey, annotationVal string) error {
	labelSelector, err := f(context, client, object)
	if err != nil {
		return err
	}
	podList, err := listPodsWithLabelSelector(context, client, object.Namespace, labelSelector)
	if err != nil {
		return err
	}
	err = annotatePods(context, client, podList, annotationKey, annotationVal)
	if err != nil {
		return err
	}
	return nil
}

func listPodsWithLabelSelector(context context.Context, client kubernetes.Interface, namespace string, labelSelector string) (*v1.PodList, error) {
	podList, err := client.CoreV1().Pods(namespace).List(context, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

func annotatePods(context context.Context, kubeClient kubernetes.Interface, podList *v1.PodList, key string, timeout string) error {
	for _, pod := range podList.Items {
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[key] = timeout
		_, err := kubeClient.CoreV1().Pods(pod.Namespace).Update(context, &pod, metav1.UpdateOptions{}) //nolint:gosec
		if err != nil {
			return err
		}
	}
	return nil
}

func getJobLabelSelector(context context.Context, client kubernetes.Interface, object CustomObject) (string, error) {
	job, err := client.BatchV1().Jobs(object.Namespace).Get(context, object.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	labelSelector := buildLabelSelector(job.Spec.Selector.MatchLabels)
	return labelSelector, nil
}

func getDeploymentLabelSelector(context context.Context, client kubernetes.Interface, object CustomObject) (string, error) {
	deployment, err := client.AppsV1().Deployments(object.Namespace).Get(context, object.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	labelSelector := buildLabelSelector(deployment.Spec.Selector.MatchLabels)
	return labelSelector, nil
}

func buildLabelSelector(labels map[string]string) string {
	var labelSelector string
	for k, v := range labels {
		labelSelector = fmt.Sprintf("%s=%s", k, v)
	}
	return labelSelector
}

func getDaemonSetLabelSelector(context context.Context, client kubernetes.Interface, object CustomObject) (string, error) {
	deployment, err := client.AppsV1().DaemonSets(object.Namespace).Get(context, object.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	labelSelector := buildLabelSelector(deployment.Spec.Selector.MatchLabels)
	return labelSelector, nil
}

func getReplicaSetLabelSelector(context context.Context, client kubernetes.Interface, object CustomObject) (string, error) {
	deployment, err := client.AppsV1().ReplicaSets(object.Namespace).Get(context, object.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	labelSelector := buildLabelSelector(deployment.Spec.Selector.MatchLabels)
	return labelSelector, nil
}

func getStatefulSetLabelSelector(context context.Context, client kubernetes.Interface, object CustomObject) (string, error) {
	deployment, err := client.AppsV1().StatefulSets(object.Namespace).Get(context, object.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	labelSelector := buildLabelSelector(deployment.Spec.Selector.MatchLabels)
	return labelSelector, nil
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
