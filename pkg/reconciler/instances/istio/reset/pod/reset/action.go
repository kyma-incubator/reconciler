package reset

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	k8sRetry "k8s.io/client-go/util/retry"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"

	v1 "k8s.io/api/core/v1"
)

//go:generate mockery --name=Action --outpkg=mocks --case=underscore
type Action interface {
	Reset(context context.Context, kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions) error
	LabelWithWarning(context context.Context, kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions) error
}

// DefaultResetAction assigns pods to handlers and executes them
type DefaultResetAction struct {
	matcher pod.Matcher
}

func NewDefaultPodsResetAction(matcher pod.Matcher) *DefaultResetAction {
	return &DefaultResetAction{
		matcher: matcher,
	}
}

func (i *DefaultResetAction) Reset(context context.Context, kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions) error {
	pods := data.RemoveAnnotatedPods(podsList, pod.AnnotationResetWarningKey)
	handlersMap := i.matcher.GetHandlersMap(kubeClient, retryOpts, pods, log, debug, waitOpts)
	g, ctx := errgroup.WithContext(context)
	for handler := range handlersMap {
		for _, object := range handlersMap[handler] {
			handler := handler
			object := object
			g.Go(func() error {
				err := handler.ExecuteAndWaitFor(ctx, object)
				if err != nil {
					return err
				}
				return nil
			})
		}
	}
	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (i *DefaultResetAction) LabelWithWarning(context context.Context, kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions) error {
	labelPatch := `{"metadata": {"labels": {"kyma-warning": "pod not in istio mesh"}}}`
	for _, podToLabel := range podsList.Items {
		err := k8sRetry.RetryOnConflict(k8sRetry.DefaultRetry, func() error {
			log.Debugf("Patching pod %s in %s namespace with label kyma-warning: pod not in istio mesh", podToLabel.Name, podToLabel.Namespace)
			_, err := kubeClient.CoreV1().Pods(podToLabel.Namespace).Patch(context, podToLabel.Name, types.MergePatchType, []byte(labelPatch), metav1.PatchOptions{})
			if err != nil {
				log.Warn("Could not label pod outside of istio mesh with warning")
			}
			return nil

		})
		if err != nil {
			return err
		}

	}
	return nil
}
