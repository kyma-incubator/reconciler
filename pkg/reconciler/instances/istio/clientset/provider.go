package clientset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"go.uber.org/zap"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//go:generate mockery --name=Provider --outpkg=mock --case=underscore
// Provider offers k8s ClientSet.
type Provider interface {
	// RetrieveFrom kubeconfig and return new k8s ClientSet instance.
	RetrieveFrom(kubeConfig string, log *zap.SugaredLogger) (kubernetes.Interface, error)

	// GetIstioOperator fetches IstioOperator from the cluster with kubeconfig passed as kubeConfig parameter
	// If kubeConfig is set to nil, the kubeconfig is fetched from KUBECONFIG env or if it's not set than from default kubeconfig dir
	GetIstioOperator(kubeConfig *string) (*istioOperator.IstioOperator, error)
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

func (d *DefaultProvider) GetIstioOperator(kubeConfig *string) (*istioOperator.IstioOperator, error) {
	dynamicClient, err := getDynamicClient(kubeConfig)
	if err != nil {
		return nil, err
	}
	item, err := dynamicClient.Resource(schema.GroupVersionResource{Group: "install.istio.io", Version: "v1alpha1", Resource: "istiooperators"}).Namespace("istio-system").Get(context.Background(), "installed-state-default-operator", v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("default Kyma IstioOperator CR wasn't found err=%s", err)
	}

	jsonSlice, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	operator := istioOperator.IstioOperator{}

	json.Unmarshal(jsonSlice, &operator)
	return &operator, nil
}

func loadKubeConfigOrDie(kubeConfig *string) (*rest.Config, error) {
	if kubeConfig == nil {
		if kubeconfig, ok := os.LookupEnv("KUBECONFIG"); ok {
			cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		}
	} else {
		cfg, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	if _, err := os.Stat(clientcmd.RecommendedHomeFile); os.IsNotExist(err) {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func getDynamicClient(kubeConfig *string) (dynamic.Interface, error) {
	config, err := loadKubeConfigOrDie(kubeConfig)
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}
