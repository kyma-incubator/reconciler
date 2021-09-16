package scheduler

import (
	"context"
	"testing"

	"go.uber.org/zap"

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
			Components: []keb.Component{
				{Component: "logging"},
				{Component: "monitoring"},
			},
		},
	}

	workerMock := &MockReconciliationWorker{}
	workerMock.On("Reconcile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	workerFactoryMock := &MockWorkerFactory{}
	workerFactoryMock.On("ForComponent", "CRDs").Return(workerMock, nil)
	workerFactoryMock.On("ForComponent", "logging").Return(workerMock, nil)
	workerFactoryMock.On("ForComponent", "monitoring").Return(workerMock, nil)

	sut := LocalScheduler{
		logger:        zap.NewNop().Sugar(),
		workerFactory: workerFactoryMock,
	}

	err := sut.Run(context.Background(), testCluster)
	require.NoError(t, err)

	workerFactoryMock.AssertNumberOfCalls(t, "ForComponent", 3)
	workerMock.AssertNumberOfCalls(t, "Reconcile", 3)
}

func TestLocalSchedulerOrder(t *testing.T) {
	testCases := []struct {
		summary       string
		prerequisites []string
		allComponents []string
		expectedOrder []string
	}{
		{
			summary:       "single prereq",
			prerequisites: []string{"b"},
			allComponents: []string{"CRDs", "a", "b"},
			expectedOrder: []string{"CRDs", "b", "a"},
		},
		{
			summary:       "multiple prereqs",
			prerequisites: []string{"b", "d"},
			allComponents: []string{"CRDs", "d", "a", "b"},
			expectedOrder: []string{"CRDs", "d", "b", "a"},
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
				if c != "CRDs" {
					testCluster.KymaConfig.Components = append(testCluster.KymaConfig.Components, keb.Component{Component: c})
				}
			}

			var reconciledComponents []string
			workerMock := &MockReconciliationWorker{}
			workerMock.On("Reconcile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).
				Run(func(args mock.Arguments) {
					component := args.Get(1).(*keb.Component)
					reconciledComponents = append(reconciledComponents, component.Component)
				})

			workerFactoryMock := &MockWorkerFactory{}
			for _, c := range tc.allComponents {
				workerFactoryMock.On("ForComponent", c).Return(workerMock, nil)
			}

			sut := LocalScheduler{
				logger:        zap.NewNop().Sugar(),
				prereqs:       tc.prerequisites,
				workerFactory: workerFactoryMock,
			}

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
	wsFact, err := workspace.NewFactory(nil, workspaceDir, logger.NewLogger(true))
	require.NoError(t, err)
	require.NoError(t, service.UseGlobalWorkspaceFactory(wsFact))

	//cleanup workspace
	cleanupFct := func(t *testing.T) {
		require.NoError(t, wsFact.Delete(kymaVersion))
	}
	cleanupFct(t)
	defer cleanupFct(t)

	workerFactory := newLocalWorkerFactory(
		zap.NewNop().Sugar(),
		&cluster.MockInventory{},
		NewInMemoryOperationsRegistry(),
		func(component string, msg *reconciler.CallbackMessage) {
			t.Logf("Component %s has status %s (error: %v)", component, msg.Status, msg.Error)
		})

	t.Run("Missing component reconciler", func(t *testing.T) {
		//no initialization of component reconcilers happened - reconciliation has to fail
		ls := LocalScheduler{
			logger:        zap.NewNop().Sugar(),
			workerFactory: workerFactory,
		}
		err := ls.Run(context.Background(), newCluster(t))
		require.Error(t, err)
	})

	t.Run("Happy path", func(t *testing.T) {
		initDefaultComponentReconciler(t)
		ls := LocalScheduler{
			logger:        zap.NewNop().Sugar(),
			workerFactory: workerFactory,
		}
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
			Components: []keb.Component{
				{Component: "cluster-essentials", Namespace: "kyma-system"},
				{Component: "istio", Namespace: "istio-system"},
			},
		},
	}
}

//initDefaultComponentReconciler initialises the default component reconciler during runtime.
//Attention: this is just a workaround for this test case to simulate edge-cases!
//Normally, the component reconcilers should be loaded automatically by adding following import to a Go file:
//`import _ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"`
func initDefaultComponentReconciler(t *testing.T) {
	_, err := service.NewComponentReconciler(DefaultReconciler)
	require.NoErrorf(t, err, "Could not create '%s' component reconciler: %s", DefaultReconciler, err)
}
