package label

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/pkg/errors"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/consts"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	k8sRetry "k8s.io/client-go/util/retry"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"

	v1 "k8s.io/api/core/v1"
)

//go:generate mockery --name=Action --outpkg=mocks --case=underscore
type Action interface {
	Run(ctx context.Context, logger *zap.SugaredLogger, client kubernetes.Interface, retryOpts []retry.Option, sidecarInjectionByDefaultEnabled bool) error
}

// DefaultResetAction assigns pods to handlers and executes them
type DefaultLabelAction struct {
	gatherer data.Gatherer
	matcher  pod.Matcher
}

func NewDefaultPodsLabelAction(gatherer data.Gatherer, matcher pod.Matcher) *DefaultLabelAction {
	return &DefaultLabelAction{
		gatherer: gatherer,
		matcher:  matcher,
	}
}

func labelWithWarning(context context.Context, kubeClient kubernetes.Interface, retryOpts wait.Backoff, podsList v1.PodList, log *zap.SugaredLogger) error {
	for _, podToLabel := range podsList.Items {
		if podToLabel.Namespace == consts.KymaIntegration || podToLabel.Namespace == consts.KymaSystem {
			continue
		}

		labelPatch := fmt.Sprintf(consts.LabelFormat, consts.KymaWarning, consts.NotInIstioMeshLabel)
		err := k8sRetry.RetryOnConflict(retryOpts, func() error {
			log.Debugf("Patching pod %s in %s namespace with label kyma-warning: %s", podToLabel.Name, podToLabel.Namespace, consts.NotInIstioMeshLabel)
			_, err := kubeClient.CoreV1().Pods(podToLabel.Namespace).Patch(context, podToLabel.Name, types.MergePatchType, []byte(labelPatch), metav1.PatchOptions{})
			if err != nil {
				return errors.Wrap(err, consts.ErrorCouldNotLabelWithWarning)
			}
			return nil
		})
		if err != nil {
			return err
		}

	}
	return nil
}

func (i *DefaultLabelAction) Run(ctx context.Context, logger *zap.SugaredLogger, client kubernetes.Interface, retryOpts []retry.Option, sidecarInjectionByDefaultEnabled bool) error {
	podsToLabelWithWarning, err := i.gatherer.GetPodsOutOfIstioMesh(client, retryOpts, sidecarInjectionByDefaultEnabled)

	if err != nil {
		return err
	}

	if len(podsToLabelWithWarning.Items) >= 1 {
		err = labelWithWarning(ctx, client, k8sRetry.DefaultRetry, podsToLabelWithWarning, logger)
		if err != nil {
			return errors.Wrap(err, "could not label pods with warning")
		}
	}

	return nil
}
