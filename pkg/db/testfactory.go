package db

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func NewTestConnectionFactory(t *testing.T) ConnectionFactory {
	configFile, err := test.GetConfigFile()
	require.NoError(t, err)

	connFac, err := NewConnectionFactory(configFile, false, true, true) //TODO: set reset to TRUE after further testing
	require.NoError(t, err)

	return connFac
}

func NewTestConnection(t *testing.T) Connection {
	test.IntegrationTest(t)
	connFac := NewTestConnectionFactory(t)
	conn, err := connFac.NewConnection()
	require.NoError(t, err)
	return conn
}
