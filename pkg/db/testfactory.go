package db

import (
	"fmt"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func NewTestConnectionFactory(t *testing.T) ConnectionFactory {
	fmt.Println("Before get Config File")
	configFile, err := test.GetConfigFile()
	require.NoError(t, err)
	fmt.Println("After get Config File")

	connFac, err := NewConnectionFactory(configFile, false, true)
	fmt.Println("After newconnectionfactory")
	require.NoError(t, err)

	require.NoError(t, connFac.Init(false))
	return connFac
}

func NewTestConnection(t *testing.T) Connection {
	connFac := NewTestConnectionFactory(t)
	conn, err := connFac.NewConnection()
	require.NoError(t, err)
	return conn
}
