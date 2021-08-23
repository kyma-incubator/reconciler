package db

import (
	"fmt"
	"path"

	file "github.com/kyma-incubator/reconciler/pkg/files"
)

func NewTestConnectionFactory() (ConnectionFactory, error) {
	configDir, err := resolveConfigsDir()
	if err != nil {
		return nil, err
	}
	connFac, err := NewConnectionFactory(path.Join(configDir, "reconciler-unittest.yaml"), true)
	if err != nil {
		return nil, err
	}
	return connFac, connFac.Init()
}

func resolveConfigsDir() (string, error) {
	configsDir := path.Join("..", "..", "configs")
	for i := 0; i < 2; i++ {
		if file.DirExists(configsDir) {
			return configsDir, nil
		}
		configsDir = path.Join("..", configsDir)
	}
	return "", fmt.Errorf("failed to resolve 'configs' directory")
}
