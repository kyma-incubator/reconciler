package gardener

import (
	gardenerapis "github.com/gardener/gardener/pkg/client/core/clientset/versioned/typed/core/v1beta1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func RestClientConfig(kubeconfig []byte) (*restclient.Config, error) {
	return clientcmd.RESTConfigFromKubeConfig(kubeconfig)
}

func NewClient(config *restclient.Config) (*gardenerapis.CoreV1beta1Client, error) {
	clientset, err := gardenerapis.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
