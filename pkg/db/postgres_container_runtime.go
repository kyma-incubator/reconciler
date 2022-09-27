package db

import (
	"context"
	"github.com/docker/go-connections/nat"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/pkg/errors"
	"os"
	"strconv"
)

type PostgresContainerRuntime struct {
	debug bool

	settings PostgresContainerSettings

	ContainerBootstrap
	ConnectionFactory
}

type ContainerSettings interface {
	id() string
	containerName() string
	containerImage() string
	migrationConfig() MigrationConfig
}

type PostgresContainerSettings struct {
	Name              string
	Image             string
	Config            MigrationConfig
	Host              string
	Database          string
	Port              int
	User              string
	Password          string
	UseSsl            bool
	EncryptionKeyFile EncryptionKeyFileConfig
}

// id calculates a hash for ContainerSettings based on the name and image of a directory as well as a cumulated hash
// over all files relevant for a migration
func (p PostgresContainerSettings) id() string {
	configHash, _ := file.HashDir(
		string(p.migrationConfig()),
		p.containerName()+p.containerImage(),
		file.HashFnv(".sql"),
	)
	return configHash
}

func (p PostgresContainerSettings) containerName() string {
	return p.Name
}

func (p PostgresContainerSettings) containerImage() string {
	return p.Image
}

func (p PostgresContainerSettings) migrationConfig() MigrationConfig {
	return p.Config
}

func RunPostgresContainer(ctx context.Context, settings PostgresContainerSettings, debug bool) (*PostgresContainerRuntime, error) {
	var filesErr error
	if settings.migrationConfig() != NoOpMigrationConfig {
		_, filesErr = os.Stat(string(settings.Config))
		if filesErr != nil {
			return nil, errors.Wrap(filesErr, "config file cannot be used to start postgres container")
		}
	}
	_, filesErr = os.Stat(string(settings.EncryptionKeyFile))
	if filesErr != nil {
		return nil, errors.Wrap(filesErr, "encryption key file cannot be used to start postgres container")
	}

	cont, bootstrapError := BootstrapNewPostgresContainer(ctx, settings)

	encKey, encryptError := readKeyFile(string(settings.EncryptionKeyFile))
	if encryptError != nil {
		return nil, encryptError
	}

	if bootstrapError != nil {
		return nil, bootstrapError
	}

	externalPort, portFetchError := cont.MappedPort(ctx, nat.Port(strconv.Itoa(settings.Port)))

	if portFetchError != nil {
		panic(portFetchError)
	}

	connectionFactory := postgresConnectionFactory{
		host:          settings.Host,
		port:          externalPort.Int(),
		database:      settings.Database,
		user:          settings.User,
		password:      settings.Password,
		sslMode:       settings.UseSsl,
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
