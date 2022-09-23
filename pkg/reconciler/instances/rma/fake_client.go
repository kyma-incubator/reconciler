package rma

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	helmfake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"io"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type FakeClient struct {
	clientset   *fake.Clientset
	helmStorage *storage.Storage
	log         *zap.SugaredLogger
}

func NewFakeClient(clientset *fake.Clientset) *FakeClient {

	return &FakeClient{
		clientset:   clientset,
		helmStorage: storage.Init(driver.NewMemory()),
		log:         logger.NewLogger(true),
	}
}

func (c *FakeClient) KubernetesClientSet() (kubernetes.Interface, error) {
	return c.clientset, nil
}

func (c *FakeClient) HelmActionConfiguration(namespace string) (*action.Configuration, error) {
	return &action.Configuration{
		Releases:     c.helmStorage,
		KubeClient:   &helmfake.FailingKubeClient{PrintingKubeClient: helmfake.PrintingKubeClient{Out: io.Discard}},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          c.log.Debugf,
	}, nil
}
