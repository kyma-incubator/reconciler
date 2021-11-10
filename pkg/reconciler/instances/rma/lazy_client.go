package rma

import (
	"sync"

	reconK8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"go.uber.org/zap"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KubeClient interface {
	ClientSet() (kubernetes.Interface, error)
	RESTClientGetter() (genericclioptions.RESTClientGetter, error)
}

type LazyKubeClient struct {
	client      kubernetes.Interface
	clientErr   error
	configFlags *genericclioptions.ConfigFlags
	initClient  sync.Once
	log         *zap.SugaredLogger
}

func (c *LazyKubeClient) init() error {
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

func (c *LazyKubeClient) ClientSet() (kubernetes.Interface, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.client, nil
}

func (c *LazyKubeClient) RESTClientGetter() (genericclioptions.RESTClientGetter, error) {
	if err := c.init(); err != nil {
		return nil, err
	}

	return c.configFlags, nil
}
