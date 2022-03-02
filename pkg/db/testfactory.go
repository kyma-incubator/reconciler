package db

import (
	"context"
	"github.com/docker/go-connections/nat"
	"github.com/spf13/viper"
	"strconv"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func NewTestConnectionFactory(t *testing.T) ConnectionFactory {
	configFile, err := test.GetConfigFile()
	require.NoError(t, err)

	connFac, err := NewConnectionFactory(configFile, false, true)
	require.NoError(t, err)

	require.NoError(t, connFac.Init(false))
	return connFac
}

func NewTestConnection(t *testing.T) Connection {
	test.IntegrationTest(t)
	connFac := NewTestConnectionFactory(t)
	conn, err := connFac.NewConnection()
	require.NoError(t, err)
	return conn
}

func NewTestPostgresContainer(t *testing.T, debug bool, migrate bool) (ContainerBootstrap, ConnectionFactory) {
	configFile, err := test.GetConfigFile()
	require.NoError(t, err)

	viper.SetConfigFile(configFile)

	configReadError := viper.ReadInConfig()
	require.NoError(t, configReadError)

	encKey, encryptError := readEncryptionKey()
	require.NoError(t, encryptError)

	env := getPostgresEnvironment()

	cont := createPostgresContainer(env)

	require.NoError(t, cont.Bootstrap(context.Background()))

	externalPort, portFetchError := cont.Container.MappedPort(context.Background(), nat.Port(strconv.Itoa(env.port)))
	require.NoError(t, portFetchError)

	connFac := &postgresConnectionFactory{
		host:          env.host,
		port:          externalPort.Int(),
		database:      env.database,
		user:          env.user,
		password:      env.password,
		sslMode:       env.sslMode,
		encryptionKey: encKey,
		migrationsDir: env.migrationsDir,
		blockQueries:  true,
		logQueries:    true,
		debug:         debug,
	}

	require.NoError(t, connFac.Init(migrate))

	return &cont, connFac
}
