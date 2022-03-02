package db

import (
	"context"
	"github.com/docker/go-connections/nat"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/spf13/viper"
	"strconv"
)

type PostgresContainerRuntime struct {
	debug   bool
	migrate bool

	env    postgresEnvironment
	encKey string

	ContainerBootstrap
	ConnectionFactory
}

func RunPostgresContainer(
	debug bool,
	migrate bool,
	ctx context.Context,
) (*PostgresContainerRuntime, error) {
	configFile, err := test.GetConfigFile()

	if err != nil {
		return nil, err
	}

	viper.SetConfigFile(configFile)

	configReadError := viper.ReadInConfig()

	if configReadError != nil {
		return nil, err
	}

	encKey, encryptError := readEncryptionKey()

	if encryptError != nil {
		return nil, err
	}

	env := getPostgresEnvironment()

	cont, bootstrapError := BootstrapNewPostgresContainer(env, ctx)

	if bootstrapError != nil {
		return nil, err
	}

	externalPort, portFetchError := cont.MappedPort(ctx, nat.Port(strconv.Itoa(env.port)))

	if portFetchError != nil {
		panic(portFetchError)
	}

	connectionFactory := postgresConnectionFactory{
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

	if initError := connectionFactory.Init(migrate); initError != nil {
		panic(initError)
	}

	return &PostgresContainerRuntime{
		debug:              debug,
		migrate:            migrate,
		env:                env,
		encKey:             encKey,
		ContainerBootstrap: cont,
		ConnectionFactory:  &connectionFactory,
	}, nil
}
