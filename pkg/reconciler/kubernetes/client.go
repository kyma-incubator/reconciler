package kubernetes

import (
	"context"
	batchv1 "k8s.io/api/batch/v1"

	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type ResourceInterceptor interface {
	Intercept(resources *ResourceList, namespace string) error
}

//go:generate mockery --name Client
type Client interface {
	Kubeconfig() string
	DeleteResource(kind, name, namespace string) (*Resource, error)
	Deploy(ctx context.Context, manifest, namespace string, interceptors ...ResourceInterceptor) ([]*Resource, error)
	Delete(ctx context.Context, manifest, namespace string) ([]*Resource, error)
	PatchUsingStrategy(kind, name, namespace string, p []byte, strategy types.PatchType) error
	Clientset() (kubernetes.Interface, error)

	GetStatefulSet(ctx context.Context, name, namespace string) (*v1apps.StatefulSet, error)
	GetSecret(ctx context.Context, name, namespace string) (*v1.Secret, error)
	GetService(ctx context.Context, name, namespace string) (*v1.Service, error)
	GetPod(ctx context.Context, name, namespace string) (*v1.Pod, error)
	GetJob(ctx context.Context, name, namespace string) (*batchv1.Job, error)
	GetPersistentVolumeClaim(ctx context.Context, name, namespace string) (*v1.PersistentVolumeClaim, error)
	ListResource(resource string, lo metav1.ListOptions) (*unstructured.UnstructuredList, error)

	GetHost() string
}
