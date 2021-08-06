package kubernetes

import (
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Resource struct {
	Kind      string
	Name      string
	Namespace string
}

func (r *Resource) String() string {
	return fmt.Sprintf("Resource [Kind:%s,Namespace:%s,Name:%s]", r.Kind, r.Namespace, r.Name)
}

type ResourceInterceptor interface {
	Intercept(resource *unstructured.Unstructured) error
}

type Client interface {
	Deploy(manifest string, interceptors ...ResourceInterceptor) ([]*Resource, error)
	Delete(manifest string) error
	Clientset() (kubernetes.Interface, error)
	Config() *rest.Config
}

func NewKubernetesClient(kubeconfig string, logger *zap.SugaredLogger) (Client, error) {
	return newKubeClientAdapter(kubeconfig, logger)
}
