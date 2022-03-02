package db

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRunPostgresTestContainer(t *testing.T) {
	t.Parallel()
	test.IntegrationTest(t)
	ctx := context.Background()
	a := require.New(t)
	runtime, runErr := RunPostgresContainer(true, false, ctx)
	a.NoError(runErr)
	a.NotNil(runtime.ContainerBootstrap.isBootstrapped(), "container should be initialized")
	a.NotNil(runtime.ContainerBootstrap.GetContainerID(), "container should be present")
	containerState, stateFetchErr := runtime.ContainerBootstrap.State(ctx)
	a.NoError(stateFetchErr)
	a.True(containerState.Running, "container should be running")
	a.NotNil(runtime.ConnectionFactory)
	a.NoError(runtime.Terminate(ctx))
}
