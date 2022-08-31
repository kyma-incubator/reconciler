package proxy

import (
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/reset"
	"github.com/pkg/errors"
	k8sRetry "k8s.io/client-go/util/retry"
)

// IstioProxyReset performs istio proxy containers reset on objects in the k8s cluster.
//
//go:generate mockery --name=IstioProxyReset --outpkg=mocks --case=underscore
type IstioProxyReset interface {
	// Run istio proxy containers reset using the config.
	Run(cfg config.IstioProxyConfig) error
}

// DefaultIstioProxyReset provides a default implementation of the IstioProxyReset.
type DefaultIstioProxyReset struct {
	gatherer data.Gatherer
	action   reset.Action
}

// NewDefaultIstioProxyReset creates a new instance of IstioProxyReset.
func NewDefaultIstioProxyReset(gatherer data.Gatherer, action reset.Action) *DefaultIstioProxyReset {
	return &DefaultIstioProxyReset{
		gatherer: gatherer,
		action:   action,
	}
}

func (i *DefaultIstioProxyReset) Run(cfg config.IstioProxyConfig) error {
	image := data.ExpectedImage{
		Prefix:  cfg.ImagePrefix,
		Version: cfg.ImageVersion,
	}

	waitOpts := pod.WaitOptions{
		Interval: cfg.Interval,
		Timeout:  cfg.Timeout,
	}

	retryOpts := []retry.Option{
		retry.Delay(cfg.DelayBetweenRetries),
		retry.Attempts(uint(cfg.RetriesCount)),
		retry.DelayType(retry.FixedDelay),
	}

	if cfg.IsUpdate {
		pods, err := i.gatherer.GetAllPods(cfg.Kubeclient, retryOpts)
		if err != nil {
			return err
		}
		cfg.Log.Debugf("Found %d pods in total", len(pods.Items))
		podsWithDifferentImage := i.gatherer.GetPodsWithDifferentImage(*pods, image)

		cfg.Log.Debugf("Found %d pods with different istio proxy image (%s)", len(podsWithDifferentImage.Items), image)
		podsWithoutAnnotation := data.RemoveAnnotatedPods(podsWithDifferentImage, pod.AnnotationResetWarningKey)
		if len(podsWithDifferentImage.Items) >= 1 && len(podsWithoutAnnotation.Items) == 0 {
			cfg.Log.Warnf(
				"Found %d pods with different istio proxy image, but we cannot update sidecar proxy image for them. Look for pods with annotation %s,"+
					" resolve the problem and remove the annotation",
				len(podsWithDifferentImage.Items),
				pod.AnnotationResetWarningKey,
			)
		}
		if len(podsWithoutAnnotation.Items) >= 1 {
			err = i.action.Reset(cfg.Context, cfg.Kubeclient, retryOpts, podsWithoutAnnotation, cfg.Log, cfg.Debug, waitOpts)
			if err != nil {
				return err
			}
			cfg.Log.Infof("Proxy reset for %d pods successfully done", len(podsWithoutAnnotation.Items))
		}
	}

	podsWithoutSidecar, podsToLabelWithWarning, err := i.gatherer.GetPodsWithoutSidecar(cfg.Kubeclient, retryOpts, cfg.SidecarInjectionByDefaultEnabled)
	if err != nil {
		return err
	}
	cfg.Log.Debugf("Found %d pods without sidecar", len(podsWithoutSidecar.Items))

	if len(podsWithoutSidecar.Items) >= 1 {
		err = i.action.Reset(cfg.Context, cfg.Kubeclient, retryOpts, podsWithoutSidecar, cfg.Log, cfg.Debug, waitOpts)
		if err != nil {
			return err
		}
		cfg.Log.Infof("Proxy reset for %d pods without sidecar successfully done", len(podsWithoutSidecar.Items))
	}

	if len(podsToLabelWithWarning.Items) >= 1 {
		err = i.action.LabelWithWarning(cfg.Context, cfg.Kubeclient, k8sRetry.DefaultRetry, podsToLabelWithWarning, cfg.Log)
		if err != nil {
			return errors.Wrap(err, "could not label pods with warning")
		}
		cfg.Log.Infof("%d pods outside of istio mesh labeled with warning", len(podsWithoutSidecar.Items))
	}

	return nil
}
