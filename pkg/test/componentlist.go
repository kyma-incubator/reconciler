package test

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"testing"
)

const (
	kymaNamespace = "kyma-system"
)

type ComponentList struct {
	DefaultNamespace string `yaml:"defaultNamespace" json:"defaultNamespace"`
	Prerequisites    []Component
	Components       []Component
}

type Component struct {
	Name      string
	Namespace string
}

func NewComponentList(t *testing.T, compListFile string) *ComponentList {
	data, err := ioutil.ReadFile(compListFile)
	require.NoError(t, err)

	compList := &ComponentList{
		DefaultNamespace: kymaNamespace,
	}
	require.NoError(t, yaml.Unmarshal(data, &compList))
	return compList
}
