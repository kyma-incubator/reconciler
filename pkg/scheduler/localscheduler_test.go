package scheduler

import (
	"context"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/test"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	kymaVersion  = "main"
	workspaceDir = "test"
)

func TestLocalScheduler(t *testing.T) {
	cluster := keb.Cluster{
		KymaConfig: keb.KymaConfig{
			Components: []keb.Components{
				{Component: "logging"},
				{Component: "monitoring"},
			},
		},
	}

	workerMock := &MockReconciliationWorker{}
	workerMock.On("Reconcile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	workerFactoryMock := &MockWorkerFactory{}
	workerFactoryMock.On("ForComponent", "logging").Return(workerMock, nil)
	workerFactoryMock.On("ForComponent", "monitoring").Return(workerMock, nil)

	sut := NewLocalScheduler(workerFactoryMock)

	err := sut.Run(context.Background(), cluster)
	require.NoError(t, err)

	workerFactoryMock.AssertNumberOfCalls(t, "ForComponent", 2)
	workerMock.AssertNumberOfCalls(t, "Reconcile", 2)
}

func TestLocalSchedulerWithKubeCluster(t *testing.T) {
	test.IntegrationTest(t)

	//use a global workspace factory to ensure all component-reconcilers are using the same workspace-directory
	//(otherwise each component-reconciler would handle the download of Kyma resources individually which will cause
	//collisions when sharing the same directory)
	wsFact, err := workspace.NewFactory(workspaceDir, logger.NewOptionalLogger(true))
	require.NoError(t, err)
	require.NoError(t, service.UseGlobalWorkspaceFactory(wsFact))

	//cleanup workspace
	cleanupFct := func(t *testing.T) {
		require.NoError(t, wsFact.Delete(kymaVersion))
	}
	cleanupFct(t)
	defer cleanupFct(t)

	t.Run("Missing component reconciler", func(t *testing.T) {
		//no initialization of component reconcilers happened - reconciliation has to fail
		ls := NewLocalScheduler(newWorkerFactory(t))
		err := ls.Run(context.Background(), newCluster(t))
		require.Error(t, err)
	})

	t.Run("Happy path", func(t *testing.T) {
		initDefaultComponentReconciler(t)
		ls := NewLocalScheduler(newWorkerFactory(t))
		err := ls.Run(context.Background(), newCluster(t))
		require.NoError(t, err)
	})

}

func newCluster(t *testing.T) keb.Cluster {
	return keb.Cluster{
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
}

func newWorkerFactory(t *testing.T) WorkerFactory {
	workerFactory, err := NewLocalWorkerFactory(
		&cluster.MockInventory{},
		NewDefaultOperationsRegistry(),
		func(component string, status reconciler.Status) {
			t.Logf("Component %s has status %s", component, status)
		},
		true)
	require.NoError(t, err)
	return workerFactory
}

//initDefaultComponentReconciler initialises the default component reconciler during runtime.
//Attention: this is just a workaround for this test case to simulate edge-cases!
//Normally, the component reconcilers should be loaded automatically by adding following import to a Go file:
//`import _ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"`
func initDefaultComponentReconciler(t *testing.T) {
	_, err := service.NewComponentReconciler(DefaultReconciler)
	require.NoErrorf(t, err, "Could not create '%s' component reconciler: %s", DefaultReconciler, err)
}
