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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	kymaVersion  = "main"
	workspaceDir = "test"
)

func TestLocalScheduler(t *testing.T) {
	testCluster := &keb.Cluster{
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

	err := sut.Run(context.Background(), testCluster)
	require.NoError(t, err)

	workerFactoryMock.AssertNumberOfCalls(t, "ForComponent", 2)
	workerMock.AssertNumberOfCalls(t, "Reconcile", 2)
}

func TestLocalSchedulerOrder(t *testing.T) {
	testCases := []struct {
		summary       string
		prerequisites []string
		crdComponents []string
		allComponents []string
		expectedOrder []string
	}{
		{
			summary:       "single prerequisite",
			prerequisites: []string{"b"},
			allComponents: []string{"a", "b"},
			expectedOrder: []string{"b", "a"},
		},
		{
			summary:       "multiple prereqs",
			prerequisites: []string{"b", "d"},
			allComponents: []string{"d", "a", "b"},
			expectedOrder: []string{"d", "b", "a"},
		},
		{
			summary:       "non-overlapping prereqs and crds",
			prerequisites: []string{"b", "d"},
			crdComponents: []string{"c", "e"},
			allComponents: []string{"d", "c", "a", "e", "b"},
			expectedOrder: []string{"d", "b", "c", "e", "a"},
		},
		{
			summary:       "overlapping prereqs and crds",
			prerequisites: []string{"b", "d"},
			crdComponents: []string{"c", "b"},
			allComponents: []string{"d", "c", "a", "b"},
			expectedOrder: []string{"d", "b", "c", "a"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.summary, func(t *testing.T) {
			t.Parallel()

			testCluster := &keb.Cluster{
				KymaConfig: keb.KymaConfig{},
			}
			for _, c := range tc.allComponents {
				testCluster.KymaConfig.Components = append(testCluster.KymaConfig.Components, keb.Components{Component: c})
			}

			var reconciledComponents []string
			workerMock := &MockReconciliationWorker{}
			workerMock.On("Reconcile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).
				Run(func(args mock.Arguments) {
					component := args.Get(0).(*keb.Components)
					reconciledComponents = append(reconciledComponents, component.Component)
				})

			workerFactoryMock := &MockWorkerFactory{}
			for _, c := range tc.allComponents {
				workerFactoryMock.On("ForComponent", c).Return(workerMock, nil)
			}

			sut := NewLocalScheduler(workerFactoryMock,
				WithPrerequisites(tc.prerequisites...),
				WithCRDComponents(tc.crdComponents...))

			err := sut.Run(context.Background(), testCluster)
			require.NoError(t, err)

			require.Len(t, reconciledComponents, len(tc.allComponents))
			require.Equal(t, tc.expectedOrder, reconciledComponents)
		})
	}
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

func newCluster(t *testing.T) *keb.Cluster {
	return &keb.Cluster{
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
		NewInMemoryOperationsRegistry(),
		func(component string, msg *reconciler.CallbackMessage) {
			t.Logf("Component %s has status %s (error: %v)", component, msg.Status, msg.Error)
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
