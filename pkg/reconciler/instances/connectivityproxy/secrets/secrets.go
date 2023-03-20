package secrets

import (
	"context"
	coreV1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

type SecretRepo struct {
	Namespace       string
	TargetClientSet k8s.Interface
}

func NewSecretRepo(namespace string, targetClientSet k8s.Interface) *SecretRepo {
	return &SecretRepo{
		Namespace:       namespace,
		TargetClientSet: targetClientSet,
	}
}

func (r SecretRepo) SaveIstioCASecret(name string, key string, ca []byte) error {

	if r.Namespace == "" {
		r.Namespace = "default"
	}

	secret := &coreV1.Secret{
		TypeMeta: v1.TypeMeta{Kind: "Secret"},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: r.Namespace,
		},
		Data: map[string][]byte{
			key: ca,
		},
		StringData: nil,
		Type:       coreV1.SecretTypeOpaque,
	}

	return r.upsertK8SSecret(secret)
}

func (r SecretRepo) upsertK8SSecret(secret *coreV1.Secret) error {

	err := r.TargetClientSet.CoreV1().
		Secrets(r.Namespace).
		Delete(context.Background(), secret.Name, v1.DeleteOptions{})

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	secret.ResourceVersion = ""

	_, err = r.TargetClientSet.CoreV1().
		Secrets(r.Namespace).
		Create(context.Background(), secret, v1.CreateOptions{})

	return err
}
