package proxy

import (
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/reset"
)

//go:generate mockery --name=IstioProxyReset --outpkg=mocks --case=underscore
// IstioProxyReset performs istio proxy containers reset on objects in the k8s cluster.
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

	pods, err := i.gatherer.GetAllPods(cfg.Kubeclient, retryOpts)
	if err != nil {
		return err
	}

	cfg.Log.Infof("Retrieved %d pods total from the cluster", len(pods.Items))

	podsWithDifferentImage := i.gatherer.GetPodsWithDifferentImage(*pods, image)

	cfg.Log.Infof("Found %d matching pods", len(podsWithDifferentImage.Items))

	err = i.action.Reset(cfg.Kubeclient, retryOpts, podsWithDifferentImage, cfg.Log, cfg.Debug, waitOpts)
	if err != nil {
		return err
	}

	return nil
}
