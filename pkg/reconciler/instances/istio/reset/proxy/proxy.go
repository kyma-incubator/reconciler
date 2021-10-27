package proxy

import (
	"time"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/reset"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

//go:generate mockery --name=IstioProxyReset --outpkg=mocks --case=underscore
// IstioProxyReset performs istio proxy containers reset on objects in the k8s cluster.
type IstioProxyReset interface {
	// Run istio proxy containers reset using the config.
	Run(cfg config.IstioProxyConfig) error

	// WaitUntilProxiesReady polls toget the current status of istio-proxy containers in the list of matched pods.
	WaitUntilProxiesReady(podsList v1.PodList, cfg config.IstioProxyConfig) error
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

	retryOpts := []retry.Option{
		retry.Delay(time.Duration(cfg.DelayBetweenRetries) * time.Second),
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

	i.action.Reset(cfg.Kubeclient, retryOpts, podsWithDifferentImage, cfg.Log, cfg.Debug)

	err = i.WaitUntilProxiesReady(podsWithDifferentImage, cfg)
	if err != nil {
		return err
	}

	return nil
}

func (i *DefaultIstioProxyReset) WaitUntilProxiesReady(podsList v1.PodList, cfg config.IstioProxyConfig) error {
	if len(podsList.Items) != 0 {
		cfg.Log.Info("Wait until all Istio proxies are running")

		err := wait.Poll(time.Duration(cfg.DelayBetweenRetries)*time.Second, cfg.Timeout, func() (done bool, err error) {
			ready := areProxiesReady(podsList)
			return ready, nil
		})
		if err != nil {
			return err
		}

		cfg.Log.Info("All proxies are up and running")
	}
	return nil
}

func areProxiesReady(podsList v1.PodList) bool {
	for _, pod := range podsList.Items {
		status := getContainerStatus(pod.Status.ContainerStatuses, "istio-proxy")
		if !status.Ready {
			return false
		}
	}

	return true
}

func getContainerStatus(statuses []v1.ContainerStatus, name string) v1.ContainerStatus {
	for i := range statuses {
		if statuses[i].Name == name {
			return statuses[i]
		}
	}

	return v1.ContainerStatus{}
}
