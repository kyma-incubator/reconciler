package connectivityproxy

import (
	"context"
	"io/ioutil"
	coreV1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"net/http"
)

type SecretFrom interface {
	Get() (*coreV1.Secret, error)
}

type SecretTo struct {
	Key       string
	Namespace string
	Name      string

	targetClientSet k8s.Interface
}

type SecretCopy struct {
	SecretFrom SecretFrom
	SecretTo   SecretTo
}

func (c SecretCopy) Transfer() error {
	secret, err := c.SecretFrom.Get()
	if err != nil {
		return err
	}
	err = c.SecretTo.createSecret(secret)
	if err != nil {
		return err
	}
	return nil
}

func NewFromURL(prefix string, targetClientSet k8s.Interface, configs map[string]string) *SecretCopy {
	return &SecretCopy{
		SecretFrom: &FromURL{
			URL: configs[prefix+".secret.url"],
			Key: configs[prefix+".secret.key"],
		},
		SecretTo: SecretTo{
			Namespace:       configs[prefix+".secret.namespace"],
			Name:            configs[prefix+".secret.name"],
			Key:             configs[prefix+".secret.key"],
			targetClientSet: targetClientSet,
		},
	}
}

func NewFromSecret(prefix string, targetClientSet k8s.Interface, inClusterClient k8s.Interface, configs map[string]string) *SecretCopy {
	return &SecretCopy{
		SecretFrom: &FromSecret{
			Name:      configs[prefix+".secret.name"],
			Namespace: configs[prefix+".secret.from.namespace"],
			inCluster: inClusterClient,
		},
		SecretTo: SecretTo{
			Namespace:       configs[prefix+".secret.to.namespace"],
			Name:            configs[prefix+".secret.name"],
			targetClientSet: targetClientSet,
		},
	}
}

type FromURL struct {
	URL string
	Key string
}

type FromSecret struct {
	Namespace string
	Name      string

	inCluster k8s.Interface
}

func (fu *FromURL) Get() (*coreV1.Secret, error) {
	ca, err := fu.query()
	if err != nil {
		return nil, err
	}

	return &coreV1.Secret{
		TypeMeta:   v1.TypeMeta{Kind: "Secret"},
		ObjectMeta: v1.ObjectMeta{},
		Data: map[string][]byte{
			fu.Key: ca,
		},
		StringData: nil,
		Type:       coreV1.SecretTypeOpaque,
	}, nil
}

func (fu *FromURL) query() ([]byte, error) {
	req, err := http.NewRequest("GET", fu.URL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (fs *FromSecret) Get() (*coreV1.Secret, error) {
	return fs.inCluster.CoreV1().Secrets("default").
		Get(context.Background(), fs.Name, v1.GetOptions{})
}

func (s *SecretTo) createSecret(secret *coreV1.Secret) error {
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
