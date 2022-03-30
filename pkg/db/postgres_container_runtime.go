package db

import (
	"context"
	"strconv"

	"github.com/docker/go-connections/nat"
)

type PostgresContainerRuntime struct {
	debug bool

	settings PostgresContainerSettings

	ContainerBootstrap
	ConnectionFactory
}

type ContainerSettings interface {
	containerName() string
	containerImage() string
	migrationConfig() MigrationConfig
}

type PostgresContainerSettings struct {
	name              string
	image             string
	config            MigrationConfig
	host              string
	database          string
	port              int
	user              string
	password          string
	useSsl            bool
	encryptionKeyFile string
}

func (p PostgresContainerSettings) containerName() string {
	return p.name
}

func (p PostgresContainerSettings) containerImage() string {
	return p.image
}

func (p PostgresContainerSettings) migrationConfig() MigrationConfig {
	return p.config
}

//MigrationConfig is currently just a migrationConfig directory but could be extended at will for further configuration
type MigrationConfig string

//NoOpMigrationConfig is a shortcut to not have any migrationConfig at all
var NoOpMigrationConfig MigrationConfig

func RunPostgresContainer(ctx context.Context, settings PostgresContainerSettings, debug bool) (*PostgresContainerRuntime, error) {
	cont, bootstrapError := BootstrapNewPostgresContainer(ctx, settings)

	encKey, encryptError := readKeyFile(settings.encryptionKeyFile)
	if encryptError != nil {
		return nil, encryptError
	}

	if bootstrapError != nil {
		return nil, bootstrapError
	}

	externalPort, portFetchError := cont.MappedPort(ctx, nat.Port(strconv.Itoa(settings.port)))

	if portFetchError != nil {
		panic(portFetchError)
	}

	connectionFactory := postgresConnectionFactory{
		host:          settings.host,
		port:          externalPort.Int(),
		database:      settings.database,
		user:          settings.user,
		password:      settings.password,
		sslMode:       settings.useSsl,
		encryptionKey: encKey,
		migrationsDir: string(settings.migrationConfig()),
		blockQueries:  true,
		logQueries:    true,
		debug:         debug,
	}

	shouldMigrate := len(string(settings.migrationConfig())) > 0

	if initError := connectionFactory.Init(shouldMigrate); initError != nil {
		panic(initError)
	}

	return &PostgresContainerRuntime{
		debug:              debug,
		settings:           settings,
		ContainerBootstrap: cont,
		ConnectionFactory:  &connectionFactory,
	}, nil
}
