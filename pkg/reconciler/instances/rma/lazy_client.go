package rma

import (
	reconK8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"sync"

	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type IntegrationClient interface {
	KubernetesClientSet() (kubernetes.Interface, error)
	HelmActionConfiguration(namespace string, log action.DebugLog) (*action.Configuration, error)
}

type LazyClient struct {
	client      kubernetes.Interface
	clientErr   error
	configFlags *genericclioptions.ConfigFlags
	initClient  sync.Once
	log         *zap.SugaredLogger
}

func (c *LazyClient) init() error {
	c.initClient.Do(func() {
		c.client, c.clientErr = reconK8s.NewInClusterClientSet(c.log)
		if c.clientErr != nil {
			return
		}

		// initialize RESTClientGetter
		config, err := rest.InClusterConfig()
		if err != nil {
			c.clientErr = err
			return
		}
		c.configFlags = genericclioptions.NewConfigFlags(false)
		c.configFlags.APIServer = &config.Host
		c.configFlags.BearerToken = &config.BearerToken
		c.configFlags.CAFile = &config.CAFile
	})
	return c.clientErr
}

func (c *LazyClient) KubernetesClientSet() (kubernetes.Interface, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client, nil
}

func (c *LazyClient) HelmActionConfiguration(namespace string, log action.DebugLog) (*action.Configuration, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	cfg := new(action.Configuration)
	err := cfg.Init(c.configFlags, namespace, RmiHelmDriver, log)

	return cfg, err
}
