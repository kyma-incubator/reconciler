package proxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/reset"
)

// IstioProxyReset task
type IstioProxyReset struct {
	// Cfg required to perform the reset
	cfg      config.IstioProxyConfig
	gatherer data.Gatherer
	action   reset.Action
}

func NewIstioProxyReset(cfg config.IstioProxyConfig, gatherer data.Gatherer, action reset.Action) *IstioProxyReset {
	return &IstioProxyReset{
		cfg:      cfg,
		gatherer: gatherer,
		action:   action,
	}
}

// Run proxy reset.
func (i *IstioProxyReset) Run() error {
	image := data.ExpectedImage{
		Prefix:  i.cfg.ImagePrefix,
		Version: i.cfg.ImageVersion,
	}

	pods, err := i.gatherer.GetAllPods()
	if err != nil {
		return err
	}

	i.cfg.Log.Infof("Retrieved %d pods total from the cluster", len(pods.Items))

	podsWithDifferentImage := i.gatherer.GetPodsWithDifferentImage(*pods, image)

	i.cfg.Log.Infof("Found %d matching pods", len(podsWithDifferentImage.Items))

	i.action.Reset(podsWithDifferentImage)

	return nil
}
