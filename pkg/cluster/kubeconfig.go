package cluster

import "fmt"

type KubeconfigProvider interface {
	Get() (string, error)
}

type DefaultKubeconfigProvider struct {
}

func (kp *DefaultKubeconfigProvider) Get() (string, error) {
	return "", fmt.Errorf("Not implemented yet")
}
