package db

import (
	"github.com/kyma-incubator/reconciler/pkg/test"
)

func NewTestConnectionFactory() (ConnectionFactory, error) {
	configFile, err := test.GetConfigFile()
	if err != nil {
		return nil, err
	}

	connFac, err := NewConnectionFactory(configFile, true)
	if err != nil {
		return nil, err
	}

	return connFac, connFac.Init()
}
