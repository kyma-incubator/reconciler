package test

import (
	"fmt"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"path"
)

const (
	configFile = "reconciler-unittest.yaml"
	configDir  = "configs"
)

func GetConfigFile() (string, error) {
	confDirPath := path.Join("..", configDir)
	for i := 0; i < 5; i++ { //lookup for 'configs' directory by climbing directory tree up (max 5 higher dirs)
		if file.DirExists(confDirPath) {

			configFile := path.Join(confDirPath, configFile)
			if !file.Exists(configFile) {
				return "", fmt.Errorf("could not find configuration file for unit tests: %s", configFile)
			}
			return configFile, nil
		}
		confDirPath = path.Join("..", confDirPath)
	}
	return "", fmt.Errorf("failed to resolve 'configs' directory")
}
