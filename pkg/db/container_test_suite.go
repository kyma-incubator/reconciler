package db

import (
	"context"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/testcontainers/testcontainers-go"
	"testing"
)

type ContainerTestSuite struct {
	TransactionAwareDatabaseContainerTestSuite
}

func IsolatedContainerTestSuite(t *testing.T, debug bool) *ContainerTestSuite {
	test.IntegrationTest(t)
	suite := NewManagedContainerTestSuite(debug, false, nil)
	return &suite
}

var (
	Default SharedInstance = "default-db-shared"
)

type SharedInstance string

var sharedContainerTestSuites map[SharedInstance]*ContainerTestSuite

func SharedContainerTestSuite(t *testing.T, debug bool, instance SharedInstance) *ContainerTestSuite {
	if sharedContainerTestSuites == nil {
		sharedContainerTestSuites = make(map[SharedInstance]*ContainerTestSuite)
	}
	if sharedContainerTestSuites[instance] == nil {
		sharedContainerTestSuites[instance] = IsolatedContainerTestSuite(t, debug)
	}
	return sharedContainerTestSuites[instance]
}

func NewManagedContainerTestSuite(
	debug bool,
	migrate bool,
	listener testcontainers.LogConsumer,
) ContainerTestSuite {
	newSuite := TransactionAwareDatabaseContainerTestSuite{
		Context:                           context.Background(),
		terminateContainerAfterAll:        true,
		connectionResilienceSpecification: []retry.Option{retry.Attempts(3)},
		LogConsumer:                       listener,
	}

	if runTime, runError := RunPostgresContainer(debug, migrate, newSuite); runError == nil {
		newSuite.ContainerRuntime = runTime
	} else {
		panic(runError)
	}

	return ContainerTestSuite{newSuite}
}

func NewUnmanagedContainerTestSuite(
	containerRuntime ContainerRuntime,
	listener testcontainers.LogConsumer,
	ctx context.Context,
) ContainerTestSuite {
	newSuite := TransactionAwareDatabaseContainerTestSuite{
		Context:                           ctx,
		terminateContainerAfterAll:        false,
		connectionResilienceSpecification: []retry.Option{retry.Attempts(3)},
		LogConsumer:                       listener,
	}
	newSuite.ContainerRuntime = containerRuntime

	return ContainerTestSuite{newSuite}
}
