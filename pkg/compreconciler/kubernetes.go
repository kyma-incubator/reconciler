package compreconciler

import (
	"github.com/kyma-incubator/reconciler/pkg/compreconciler/types"
	"k8s.io/client-go/kubernetes"
)

type kubernetesClient interface {
	Deploy(manifest string) ([]string, []types.Metadata, error)
	Delete(manifest string) error
	Clientset() (*kubernetes.Clientset, error)
}

func newKubernetesClient(kubeconfig string) (kubernetesClient, error) {
	return newGoClient(kubeconfig)
}
