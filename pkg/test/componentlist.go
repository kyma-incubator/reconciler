package test

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"testing"
)

const (
	kymaComponentListURL = "https://raw.githubusercontent.com/kyma-project/kyma/main/installation/resources/components.yaml"
	kymaNamespace        = "kyma-system"
)

type KymaComponentList struct {
	DefaultNamespace string `yaml:"defaultNamespace" json:"defaultNamespace"`
	Prerequisites    []Component
	Components       []Component
}

type ComponentList struct {
	Prerequisites []Component
	Components    []Component
}

type Component struct {
	Name      string
	Namespace string
}

func NewKymaComponentList(t *testing.T) *KymaComponentList {
	payload := httpGET(t)

	compList := &KymaComponentList{
		DefaultNamespace: kymaNamespace,
	}
	require.NoError(t, yaml.Unmarshal(payload, &compList))
	return compList
}

func httpGET(t *testing.T) []byte {
	//get latest Kyma component list from Github
	resp, err := http.Get(kymaComponentListURL)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	payload, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return payload
}
