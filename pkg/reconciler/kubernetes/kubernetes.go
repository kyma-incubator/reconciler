package kubernetes

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
)

type Resource struct {
	Kind      string
	Name      string
	Namespace string
}

func (r *Resource) String() string {
	return fmt.Sprintf("Resource [Kind:%s,Namespace:%s,Name:%s]", r.Kind, r.Namespace, r.Name)
}

type KubernetesClient interface {
	Deploy(manifest string) error
	DeployedResources(manifest string) ([]Resource, error)
	Delete(manifest string) error
	Clientset() (*kubernetes.Clientset, error)
}

func NewKubernetesClient(kubeconfig string) (KubernetesClient, error) {
	return newKubectlClient(kubeconfig)
}
