package kubernetes

import (
	"context"
	"fmt"

	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type Resource struct {
	Kind      string
	Name      string
	Namespace string
}

func (r *Resource) String() string {
	return fmt.Sprintf("KubernetesResource [Kind:%s,Namespace:%s,Name:%s]", r.Kind, r.Namespace, r.Name)
}

type ResourceInterceptor interface {
	Intercept(resource *unstructured.Unstructured) error
}

//go:generate mockery --name Client
type Client interface {
	Kubeconfig() string
	Deploy(ctx context.Context, manifest, namespace string, interceptors ...ResourceInterceptor) ([]*Resource, error)
	Delete(ctx context.Context, manifest, namespace string) ([]*Resource, error)
	PatchUsingStrategy(kind, name, namespace string, p []byte, strategy types.PatchType) error
	Clientset() (kubernetes.Interface, error)

	GetStatefulSet(ctx context.Context, name, namespace string) (*v1apps.StatefulSet, error)
	GetSecret(ctx context.Context, name, namespace string) (*v1.Secret, error)
	ListResource(resource string, lo metav1.ListOptions) (*unstructured.UnstructuredList, error)
}
