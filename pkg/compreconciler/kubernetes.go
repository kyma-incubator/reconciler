package compreconciler

import (
	"k8s.io/client-go/kubernetes"
)

type resource struct {
	kind      string
	name      string
	namespace string
}

type kubernetesClient interface {
	Deploy(manifest string) error
	DeployedResources(manifest string) ([]resource, error)
	Delete(manifest string) error
	Clientset() (*kubernetes.Clientset, error)
}

func newKubernetesClient(kubeconfig string) (kubernetesClient, error) {
	return newKubectlClient(kubeconfig)
}
