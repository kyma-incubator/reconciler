package clientset

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

//go:generate mockery --name=Provider --outpkg=mock --case=underscore
// Provider offers k8s ClientSet.
type Provider interface {
	// RetrieveFrom kubeconfig and return new k8s ClientSet instance.
	RetrieveFrom(kubeConfig string, log *zap.SugaredLogger) (kubernetes.Interface, error)
}

// DefaultProvider provides a default implementation of Provider.
type DefaultProvider struct{}

func (c *DefaultProvider) RetrieveFrom(kubeConfig string, log *zap.SugaredLogger) (kubernetes.Interface, error) {
	kubeconfigPath, kubeconfigCf, err := file.CreateTempFileWith(kubeConfig)
	if err != nil {
		return nil, err
	}

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			log.Error(cleanupErr)
		}
	}()

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return kubeClient, nil
}
