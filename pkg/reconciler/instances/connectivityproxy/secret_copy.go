package connectivityproxy

import (
	"context"
	coreV1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

type SecretFrom interface {
	Get() (*coreV1.Secret, error)
}

type SecretCopy struct {
	Namespace string
	Name      string

	targetClientSet k8s.Interface
	from            SecretFrom
}

func (c *SecretCopy) Transfer() error {
	secret, err := c.from.Get()
	if err != nil {
		return err
	}
	err = c.createSecret(secret)
	if err != nil {
		return err
	}
	return nil
}

func (s *SecretCopy) createSecret(secret *coreV1.Secret) error {
	err := s.targetClientSet.CoreV1().
		Secrets(s.Namespace).
		Delete(context.Background(), s.Name, v1.DeleteOptions{})

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	secret.ResourceVersion = ""
	secret.Namespace = s.Namespace
	secret.Name = s.Name

	_, err = s.targetClientSet.CoreV1().
		Secrets(s.Namespace).
		Create(context.Background(), secret, v1.CreateOptions{})

	return err
}
