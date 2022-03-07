package db

import (
	"context"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/testcontainers/testcontainers-go"
	"path/filepath"
	"sync"
	"testing"
)

type ContainerTestSuite struct {
	TransactionAwareDatabaseContainerTestSuite
}

type SharedContainerSettings struct {
	name string
	MigrationConfig
}

type SyncedSharedContainerTestSuiteInstanceHolder struct {
	mu     sync.Mutex
	suites map[SharedContainerSettings]*ContainerTestSuite
}

func IsolatedContainerTestSuite(t *testing.T, debug bool, migrations MigrationConfig) *ContainerTestSuite {
	test.IntegrationTest(t)
	suite := NewManagedContainerTestSuite(debug, migrations, NewConsoleContainerLogListener(debug))
	return &suite
}

var (
	Default = SharedContainerSettings{
		"default-db-shared",
		MigrationConfig(filepath.Join("..", "..", "configs", "db", "postgres")),
	}
	syncedSharedContainerTestSuiteInstanceHolder *SyncedSharedContainerTestSuiteInstanceHolder
)

func SharedContainerTestSuite(t *testing.T, debug bool, instance SharedContainerSettings) *ContainerTestSuite {
	h := syncedSharedContainerTestSuiteInstanceHolder
	if h == nil {
		h = &SyncedSharedContainerTestSuiteInstanceHolder{
			mu:     sync.Mutex{},
			suites: make(map[SharedContainerSettings]*ContainerTestSuite),
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.suites[instance] == nil {
		h.suites[instance] = IsolatedContainerTestSuite(t, debug, instance.MigrationConfig)
	}
	return h.suites[instance]
}

func NewManagedContainerTestSuite(
	debug bool,
	migrations MigrationConfig,
	listener testcontainers.LogConsumer,
) ContainerTestSuite {
	newSuite := TransactionAwareDatabaseContainerTestSuite{
		Context:                           context.Background(),
		terminateContainerAfterAll:        true,
		connectionResilienceSpecification: []retry.Option{retry.Attempts(3)},
		LogConsumer:                       listener,
	}

	if runTime, runError := RunPostgresContainer(newSuite, migrations, debug); runError == nil {
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
