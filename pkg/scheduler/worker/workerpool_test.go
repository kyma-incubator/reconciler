package worker

import (
	"context"
	"github.com/pkg/errors"
	"sync"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/invoker"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

type testInvoker struct {
	params     []*invoker.Params
	reconRepo  reconciliation.Repository
	mux        sync.Mutex
	errChannel chan error
}

func (i *testInvoker) Invoke(_ context.Context, params *invoker.Params) error {
	if err := i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID, model.OperationStateInProgress, ""); err != nil {
		i.errChannel <- errors.New("Update failed")
		return err
	}
	i.mux.Lock()
	i.params = append(i.params, params)
	i.mux.Unlock()
	return nil
}

func TestWorkerPool(t *testing.T) {
	//test.IntegrationTest(t) //required because a valid Kubeconfig is required to create test cluster entry

	//create cluster inventory
	inventory, err := cluster.NewInventory(db.NewTestConnection(t), true, &cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	//add cluster to inventory
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Administrators: nil,
			Components: []keb.Component{
				{
					Component: "TestComp1",
				},
			},
			Profile: "",
			Version: "1.2.3",
		},
		Metadata:     keb.Metadata{},
		RuntimeID:    "testCluster",
		RuntimeInput: keb.RuntimeInput{},
	})
	require.NoError(t, err)

	//cleanup created cluster
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
	}()

	//create test invoker to be able to verify invoker calls
	testInvoker := &testInvoker{}

	//create reconciliation for cluster
	testInvoker.reconRepo = reconciliation.NewInMemoryReconciliationRepository()
	reconEntity, err := testInvoker.reconRepo.CreateReconciliation(clusterState, nil)
	require.NoError(t, err)
	opsProcessable, err := testInvoker.reconRepo.GetProcessableOperations(0)
	require.Len(t, opsProcessable, 1)
	require.NoError(t, err)

	//start worker pool
	workerPool, err := NewWorkerPool(&InventoryRetriever{inventory}, testInvoker.reconRepo, testInvoker, nil, logger.NewLogger(true))
	require.NoError(t, err)

	//create time limited context
	ctx, cancelFct := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelFct()

	//ensure worker pool stops when context gets closed
	startTime := time.Now()
	require.NoError(t, workerPool.Run(ctx))
	require.WithinDuration(t, startTime, time.Now(), 3*time.Second) //ensure workerPool is considering ctx

	//verify that invoker was properly called
	require.Len(t, testInvoker.params, 1)
	require.Equal(t, clusterState, testInvoker.params[0].ClusterState)
	require.Equal(t, model.CRDComponent, testInvoker.params[0].ComponentToReconcile.Component) //CRDs is always the first component
	require.Equal(t, reconEntity.SchedulingID, testInvoker.params[0].SchedulingID)
	require.Equal(t, opsProcessable[0].CorrelationID, testInvoker.params[0].CorrelationID)
}

func TestWorkerPoolParallel(t *testing.T) {

	t.Run("Multiple WorkerPools watching same reconciliation repository", func(t *testing.T) {

		//initialize WaitGroup
		var wg sync.WaitGroup
		//prepare keb clusters
		kebClusters := []*keb.Cluster{
			{
				Kubeconfig: "clusterA",
				KymaConfig: keb.KymaConfig{
					Administrators: nil,
					Components: []keb.Component{
						{
							Component: "TestComp1",
						},
					},
					Profile: "",
					Version: "1.2.3",
				},
				Metadata:     keb.Metadata{},
				RuntimeID:    "testClusterA",
				RuntimeInput: keb.RuntimeInput{},
			},
			{
				Kubeconfig: "clusterB",
				KymaConfig: keb.KymaConfig{
					Administrators: nil,
					Components: []keb.Component{
						{
							Component: "TestComp1",
						},
					},
					Profile: "",
					Version: "1.2.3",
				},
				Metadata:     keb.Metadata{},
				RuntimeID:    "testClusterB",
				RuntimeInput: keb.RuntimeInput{},
			},
			{
				Kubeconfig: "clusterC",
				KymaConfig: keb.KymaConfig{
					Administrators: nil,
					Components: []keb.Component{
						{
							Component: "TestComp3",
						},
					},
					Profile: "",
					Version: "1.2.3",
				},
				Metadata:     keb.Metadata{},
				RuntimeID:    "testClusterC",
				RuntimeInput: keb.RuntimeInput{},
			},
		}

		//create mock database connection
		testDB := db.NewTestConnection(t)

		//create cluster inventory
		inventory, err := cluster.NewInventory(testDB, true, &cluster.MetricsCollectorMock{})
		require.NoError(t, err)

		//add clusters to inventory
		var clusterStates [3]*cluster.State
		for i := range kebClusters {
			clusterStates[i], err = inventory.CreateOrUpdate(1, kebClusters[i])
		}
		require.NoError(t, err)

		//create test invoker to be able to verify invoker calls and update operation state
		testInvoker := &testInvoker{errChannel: make(chan error, 100)}

		//create reconciliation for cluster
		testInvoker.reconRepo, err = reconciliation.NewPersistedReconciliationRepository(testDB, true)
		require.NoError(t, err)
		for i := range clusterStates {
			_, err = testInvoker.reconRepo.CreateReconciliation(clusterStates[i], nil)
		}
		require.NoError(t, err)
		opsProcessable, err := testInvoker.reconRepo.GetProcessableOperations(0)
		require.Len(t, opsProcessable, 3) // only first prio
		require.NoError(t, err)

		//initialize worker pool
		workerPool, err := NewWorkerPool(&InventoryRetriever{inventory}, testInvoker.reconRepo, testInvoker, nil, logger.NewLogger(true))
		require.NoError(t, err)

		//create time limited context
		ctx, cancelFct := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelFct()

		//start multiple worker pools
		errChannel := make(chan error)
		startAt := time.Now().Add(1 * time.Second)
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(time.Until(startAt))
				err := workerPool.Run(ctx)
				if err != nil {
					errChannel <- err
				}
			}()
		}
		wg.Wait()

		//verify that invoker was properly called
		require.Equal(t, 12, len(testInvoker.errChannel))
		require.Len(t, testInvoker.params, 3)
		for i := 0; i < 3; i++ {
			require.Equal(t, model.ClusterStatusReconcilePending, testInvoker.params[i].ClusterState.Status.Status)
			require.Equal(t, model.CRDComponent, testInvoker.params[i].ComponentToReconcile.Component)
		}
	})
}
