package components

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

const (
	KymaNamespace = "kyma-system"
)

type ComponentList struct {
	DefaultNamespace string `yaml:"defaultNamespace" json:"defaultNamespace"`
	Prerequisites    []Component
	Components       []Component
}

type Component struct {
	Name          string
	Namespace     string
	URL           string
	Configuration map[string]interface{}
	Version       string
}

func NewComponentList(compListFile string) (*ComponentList, error) {
	data, err := ioutil.ReadFile(compListFile)
	if err != nil {
		return nil, err
	}

	compList := &ComponentList{
		DefaultNamespace: KymaNamespace,
	}

	if err := yaml.Unmarshal(data, &compList); err != nil {
		return nil, err
	}

	return compList, nil
}
