package db

import (
	"context"
	"github.com/docker/go-connections/nat"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/spf13/viper"
	"strconv"
)

type PostgresContainerRuntime struct {
	debug bool

	env    postgresEnvironment
	encKey string

	ContainerBootstrap
	ConnectionFactory
}

//MigrationConfig is currently just a migrationConfig directory but could be extended at will for further configuration
type MigrationConfig string

//NoOpMigrationConfig is a shortcut to not have any migrationConfig at all
var NoOpMigrationConfig MigrationConfig = ""

func RunPostgresContainer(ctx context.Context, migrationConfig MigrationConfig, debug bool) (*PostgresContainerRuntime, error) {
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

	cont, bootstrapError := BootstrapNewPostgresContainer(ctx, env)

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
		migrationsDir: string(migrationConfig),
		blockQueries:  true,
		logQueries:    true,
		debug:         debug,
	}

	shouldMigrate := len(string(migrationConfig)) > 0

	if initError := connectionFactory.Init(shouldMigrate); initError != nil {
		panic(initError)
	}

	return &PostgresContainerRuntime{
		debug:              debug,
		env:                env,
		encKey:             encKey,
		ContainerBootstrap: cont,
		ConnectionFactory:  &connectionFactory,
	}, nil
}
