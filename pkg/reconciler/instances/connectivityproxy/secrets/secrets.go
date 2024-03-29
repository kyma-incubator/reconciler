package secrets

import (
	"context"

	coreV1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

const (
	TagTLSCa = "ca.crt"
)

type SecretRepo struct {
	Namespace       string
	TargetClientSet k8s.Interface
}

func NewSecretRepo(namespace string, targetClientSet k8s.Interface) *SecretRepo {

	if namespace == "" {
		namespace = "default"
	}

	return &SecretRepo{
		Namespace:       namespace,
		TargetClientSet: targetClientSet,
	}
}

func (r SecretRepo) SaveSecretCpSvcKey(ctx context.Context, name, key string) error {

	secret := &coreV1.Secret{
		TypeMeta: v1.TypeMeta{Kind: "Secret"},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: r.Namespace,
		},
		Data: map[string][]byte{
			"service_key": []byte(key),
		},
		Type: coreV1.SecretTypeOpaque,
	}

	return r.upsertK8SSecret(ctx, secret)
}

func (r SecretRepo) SaveSecretMappingOperator(ctx context.Context, name string, key, crt []byte) (map[string][]byte, error) {
	// TODO make sure to do a certificate expiration check
	secret, err := r.TargetClientSet.
		CoreV1().
		Secrets(r.Namespace).
		Get(ctx, name, v1.GetOptions{})

	if err == nil {
		return secret.Data, nil
	}

	if !k8serrors.IsNotFound(err) {
		return nil, err
	}

	secret = &coreV1.Secret{
		TypeMeta: v1.TypeMeta{Kind: "Secret"},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: r.Namespace,
		},
		Data: map[string][]byte{
			"tls.crt": crt,
			TagTLSCa:  crt,
			"tls.key": key,
		},
		Type: coreV1.SecretTypeOpaque,
	}

	// TODO switch to upsertK8SSecret when cert rotation is implemented
	_, err = r.TargetClientSet.CoreV1().
		Secrets(r.Namespace).
		Create(context.Background(), secret, v1.CreateOptions{})

	return secret.Data, err
}

func (r SecretRepo) SaveIstioCASecret(ctx context.Context, name string, key string, ca []byte) error {

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

	return r.upsertK8SSecret(ctx, secret)
}

func (r SecretRepo) upsertK8SSecret(ctx context.Context, secret *coreV1.Secret) error {

	err := r.TargetClientSet.CoreV1().
		Secrets(r.Namespace).
		Delete(ctx, secret.Name, v1.DeleteOptions{})

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	_, err = r.TargetClientSet.CoreV1().
		Secrets(r.Namespace).
		Create(ctx, secret, v1.CreateOptions{})

	return err
}
