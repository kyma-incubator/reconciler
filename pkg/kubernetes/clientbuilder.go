package kubernetes

import (
	"fmt"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

const EnvVarKubeconfig = "KUBECONFIG"

type ClientBuilder struct {
	kubeconfig []byte
	err        error
}

func (cb *ClientBuilder) WithFile(filePath string) *ClientBuilder {
	cb.kubeconfig, cb.err = cb.loadFile(filePath)
	return cb
}

func (cb *ClientBuilder) WithString(kubeconfig string) *ClientBuilder {
	cb.kubeconfig = []byte(kubeconfig)
	return cb
}

func (cb *ClientBuilder) Build() (*kubernetes.Clientset, error) {
	if cb.err != nil {
		return nil, cb.err
	}
	if len(cb.kubeconfig) == 0 {
		kubeconfigPath := os.Getenv(EnvVarKubeconfig)
		if kubeconfigPath == "" {
			return nil, fmt.Errorf("kubeconfig undefined: please provide it as file, string or set env-var %s",
				EnvVarKubeconfig)
		}
		cb.kubeconfig, cb.err = cb.loadFile(kubeconfigPath)
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(cb.kubeconfig)
	if err != nil {
		return nil, errors.Wrap(err,
			fmt.Sprintf("failed to create Kubernetes client configuration using provided kubeconfig: %s", cb.kubeconfig))
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Kubernetes clientset by using provided REST-configuration")
	}
	return clientSet, err
}

func (cb *ClientBuilder) loadFile(filePath string) ([]byte, error) {
	if !file.Exists(filePath) {
		return nil, fmt.Errorf("kubeconfig file not found at path '%s'", filePath)
	}
	return ioutil.ReadFile(filePath)
}
