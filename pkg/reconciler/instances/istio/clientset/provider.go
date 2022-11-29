package clientset

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"github.com/kyma-project/istio/operator/api/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Provider offers k8s Clients.
//
//go:generate mockery --name=Provider --outpkg=mock --case=underscore
type Provider interface {
	// RetrieveFrom kubeconfig and return new k8s ClientSet instance.
	RetrieveFrom(kubeConfig string, log *zap.SugaredLogger) (kubernetes.Interface, error)

	// GetIstioClient returns a new controller-runtime Client using provided config and Kyma Istio Operator scheme.
	GetIstioClient(kubeConfig string) (client.Client, error)

	// GetDynamicClient returns a new dynamic Kubernetes Client using provided config.
	GetDynamicClient(kubeConfig string) (dynamic.Interface, error)
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

func (c *DefaultProvider) GetIstioClient(kubeConfig string) (client.Client, error) {
	config, err := loadRestConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	err = v1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	err = corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	err = appsv1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	client, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *DefaultProvider) GetDynamicClient(kubeConfig string) (dynamic.Interface, error) {
	config, err := loadRestConfig(kubeConfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return dynamicClient, nil
}

func loadRestConfig(kubeconfigData string) (*rest.Config, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfigData))
	if err != nil {
		return nil, err
	}
	cfg.WarningHandler = rest.NoWarnings{}
	return cfg, nil
}
