package connectivityproxy

import (
	"io/ioutil"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

type FromURL struct {
	URL string
	Key string
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
	defer resp.Body.Close() // TODO: handle error
	return ioutil.ReadAll(resp.Body)
}
