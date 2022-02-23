package gardener

import (
	"context"
	"errors"
	"fmt"

	gardener_Types "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate mockery -name=ShootClient
type ShootClient interface {
	List(ctx context.Context, opts metav1.ListOptions) (*gardener_Types.ShootList, error)
}

type KubernetesVersionProvider struct {
	shootClient ShootClient
}

func NewKubernetesVersionProvider(shootClient ShootClient) KubernetesVersionProvider {
	return KubernetesVersionProvider{
		shootClient: shootClient,
	}
}

func (k KubernetesVersionProvider) Get(runtimeID string, tenant string) (string, error) {
	labelSelector := fmt.Sprintf("%s=%s", AccountLabel, tenant)

	shoots, err := k.shootClient.List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to list shoots: %s", err.Error()))
	}

	for _, shoot := range shoots.Items {
		id := shoot.Annotations[runtimeIDAnnotation]

		if id == runtimeID {
			return shoot.Spec.Kubernetes.Version, nil
		}
	}

	return "", errors.New(fmt.Sprintf("failed to find shoot for Runtime %s", runtimeID))

}
