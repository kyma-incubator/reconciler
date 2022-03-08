package db

import (
	"context"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"path/filepath"
	"sync"
	"testing"
)

type ContainerTestSuite struct {
	TransactionAwareDatabaseContainerTestSuite
}

type SyncedSharedContainerTestSuiteInstanceHolder struct {
	mu     sync.Mutex
	suites map[ContainerSettings]*ContainerTestSuite
}

func IsolatedContainerTestSuite(t *testing.T, debug bool, settings ContainerSettings) *ContainerTestSuite {
	test.IntegrationTest(t)
	suite := NewManagedContainerTestSuite(debug, settings, NewConsoleContainerLogListener(debug))
	return &suite
}

var (
	Default = PostgresContainerSettings{
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
	syncedSharedContainerTestSuiteInstanceHolder *SyncedSharedContainerTestSuiteInstanceHolder
)

func SharedContainerTestSuite(t *testing.T, debug bool, instance ContainerSettings) *ContainerTestSuite {
	h := syncedSharedContainerTestSuiteInstanceHolder
	if h == nil {
		h = &SyncedSharedContainerTestSuiteInstanceHolder{
			mu:     sync.Mutex{},
			suites: make(map[ContainerSettings]*ContainerTestSuite),
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.suites[instance] == nil {
		h.suites[instance] = IsolatedContainerTestSuite(t, debug, instance)
	}
	return h.suites[instance]
}

func NewManagedContainerTestSuite(
	debug bool,
	settings ContainerSettings,
	listener testcontainers.LogConsumer,
) ContainerTestSuite {
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

	if runTime, runError := RunPostgresContainer(newSuite, postgresSettings, debug); runError == nil {
		newSuite.ContainerRuntime = runTime
	} else {
		panic(runError)
	}

	return ContainerTestSuite{newSuite}
}

func NewUnmanagedContainerTestSuite(ctx context.Context, containerRuntime ContainerRuntime, listener testcontainers.LogConsumer) ContainerTestSuite {
	newSuite := TransactionAwareDatabaseContainerTestSuite{
		Context:                           ctx,
		terminateContainerAfterAll:        false,
		connectionResilienceSpecification: []retry.Option{retry.Attempts(3)},
		LogConsumer:                       listener,
	}
	newSuite.ContainerRuntime = containerRuntime

	return ContainerTestSuite{newSuite}
}
