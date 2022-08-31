package label

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/pkg/errors"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	k8sRetry "k8s.io/client-go/util/retry"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"

	v1 "k8s.io/api/core/v1"
)

//go:generate mockery --name=Action --outpkg=mocks --case=underscore
type Action interface {
	Run(cfg *config.IstioProxyConfig) error
	LabelWithWarning(context context.Context, kubeClient kubernetes.Interface, retryOpts wait.Backoff, podsList v1.PodList, log *zap.SugaredLogger) error
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

func (i *DefaultLabelAction) LabelWithWarning(context context.Context, kubeClient kubernetes.Interface, retryOpts wait.Backoff, podsList v1.PodList, log *zap.SugaredLogger) error {
	for _, podToLabel := range podsList.Items {
		labelPatch := fmt.Sprintf(config.LabelFormat, config.KymaWarning, config.NotInIstioMeshLabel)
		err := k8sRetry.RetryOnConflict(retryOpts, func() error {
			log.Debugf("Patching pod %s in %s namespace with label kyma-warning: %s", podToLabel.Name, podToLabel.Namespace, config.NotInIstioMeshLabel)
			_, err := kubeClient.CoreV1().Pods(podToLabel.Namespace).Patch(context, podToLabel.Name, types.MergePatchType, []byte(labelPatch), metav1.PatchOptions{})
			if err != nil {
				return errors.Wrap(err, config.ErrorCouldNotLabelWithWarning)
			}
			return nil

		})
		if err != nil {
			return err
		}

	}
	return nil
}

func (i *DefaultLabelAction) Run(cfg *config.IstioProxyConfig) error {
	retryOpts := []retry.Option{
		retry.Delay(cfg.DelayBetweenRetries),
		retry.Attempts(uint(cfg.RetriesCount)),
		retry.DelayType(retry.FixedDelay),
	}

	podsToLabelWithWarning, err := i.gatherer.GetPodsOutOfIstioMesh(cfg.Kubeclient, retryOpts, cfg.SidecarInjectionByDefaultEnabled)

	if err != nil {
		return err
	}

	if len(podsToLabelWithWarning.Items) >= 1 {
		err = i.LabelWithWarning(cfg.Context, cfg.Kubeclient, k8sRetry.DefaultRetry, podsToLabelWithWarning, cfg.Log)
		if err != nil {
			return errors.Wrap(err, "could not label pods with warning")
		}
		cfg.Log.Infof("%d pods outside of istio mesh labeled with warning", len(podsToLabelWithWarning.Items))
	}

	return nil
}
