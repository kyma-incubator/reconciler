package scheduler

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	kymaVersion  = "main"
	workspaceDir = "test"
)

func TestLocalSchedulerWithKubeCluster(t *testing.T) {
	if !test.RunIntegrationTests() {
		t.Skip("Skipping an expensive test...")
	}

	//cleanup workspace
	defer func() {
		wsDir := filepath.Join(workspaceDir, kymaVersion)
		t.Logf("Deleting cloned Kyma sources in %s", wsDir)
		require.NoError(t, os.RemoveAll(wsDir))
	}()

	initComponentReconcilers(t)

	operationsRegistry := NewDefaultOperationsRegistry()
	workerFactory := newWorkerFactory(t, operationsRegistry)
	localScheduler := newLocalScheduler(t, workerFactory)

	err := localScheduler.Run(context.Background())
	require.NoError(t, err)
}

func newLocalScheduler(t *testing.T, workerFactory WorkerFactory) Scheduler {
	kebCluster := keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Version: kymaVersion,
			Profile: "evaluation",
			Components: []keb.Components{
				{Component: "cluster-essentials", Namespace: "kyma-system"},
				{Component: "istio", Namespace: "istio-system"},
			},
		},
	}
	ls, err := NewLocalScheduler(kebCluster, workerFactory, true)
	require.NoError(t, err)
	return ls
}

func newWorkerFactory(t *testing.T, operationsRegistry *DefaultOperationsRegistry) WorkerFactory {
	workerFactory, err := NewLocalWorkerFactory(
		&cluster.MockInventory{},
		operationsRegistry,
		func(component string, status reconciler.Status) {
			t.Logf("Component %s has status %s", component, status)
		},
		true)
	require.NoError(t, err)
	return workerFactory
}

func initComponentReconcilers(t *testing.T) {
	wsFact, err := workspace.NewFactory("test", logger.NewOptionalLogger(true))
	require.NoError(t, err)
	require.NoError(t, service.UseGlobalWorkspaceFactory(wsFact))

	_, err = service.NewComponentReconciler("cluster-essentials")
	require.NoErrorf(t, err, "Could not create '%s' component reconciler: %s", "cluster-essentials", err)

	_, err = service.NewComponentReconciler("istio")
	require.NoErrorf(t, err, "Could not create '%s' component reconciler: %s", "istio", err)
}
