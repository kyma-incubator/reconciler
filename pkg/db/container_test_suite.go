package db

import (
	"context"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"path/filepath"
	"testing"
)

type ContainerTestSuite struct {
	*TransactionAwareDatabaseContainerTestSuite
}

func IsolatedContainerTestSuite(t *testing.T, debug bool, settings ContainerSettings) *ContainerTestSuite {
	test.IntegrationTest(t)
	return NewManagedContainerTestSuite(debug, settings, NewConsoleContainerLogListener(debug))
}

var (
	DefaultSharedContainerSettings = PostgresContainerSettings{
		"default-db-shared",
		"postgres:11-alpine",
		MigrationConfig(filepath.Join("..", "..", "configs", "db", "postgres")),
		"127.0.0.1",
		"kyma",
		5432,
		"kyma",
		"kyma",
		false,
		filepath.Join("..", "..", "configs", "encryption", "unittest.key"),
	}
)

func NewManagedContainerTestSuite(
	debug bool,
	settings ContainerSettings,
	listener testcontainers.LogConsumer,
) *ContainerTestSuite {
	newSuite := TransactionAwareDatabaseContainerTestSuite{
		Context:                           context.Background(),
		terminateContainerAfterAll:        true,
		connectionResilienceSpecification: []retry.Option{retry.Attempts(3)},
		LogConsumer:                       listener,
	}

	postgresSettings, settingsAreForPostgres := settings.(PostgresContainerSettings)

	if !settingsAreForPostgres {
		panic(errors.New("settings are not for postgres"))
	}

	if runTime, runError := RunPostgresContainer(newSuite.Context, postgresSettings, debug); runError == nil {
		newSuite.ContainerRuntime = runTime
	} else {
		panic(runError)
	}

	return &ContainerTestSuite{&newSuite}
}

func NewUnmanagedContainerTestSuite(ctx context.Context, containerRuntime ContainerRuntime, listener testcontainers.LogConsumer) *ContainerTestSuite {
	newSuite := TransactionAwareDatabaseContainerTestSuite{
		Context:                           ctx,
		terminateContainerAfterAll:        false,
		connectionResilienceSpecification: []retry.Option{retry.Attempts(3)},
		LogConsumer:                       listener,
	}
	newSuite.ContainerRuntime = containerRuntime

	return &ContainerTestSuite{&newSuite}
}
