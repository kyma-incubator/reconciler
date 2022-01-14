package worker

import (
	"context"
	kebTest "github.com/kyma-incubator/reconciler/pkg/keb/test"
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
	errChannel chan error
	sync.WaitGroup
}

type testInvokerParallel struct {
	params     []*invoker.Params
	reconRepo  reconciliation.Repository
	errChannel chan error
	sync.Mutex
}

func (i *testInvoker) Invoke(_ context.Context, params *invoker.Params) error {
	if err := i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID, model.OperationStateInProgress, false, ""); err != nil {
		return err
	}
	i.params = append(i.params, params)
	time.Sleep(1 * time.Second)
	i.Done()
	return nil
}

func (i *testInvokerParallel) Invoke(_ context.Context, params *invoker.Params) error {
	if err := i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID, model.OperationStateInProgress, false, ""); err != nil {
		i.errChannel <- errors.New("Update failed")
		return err
	}
	i.Lock()
	i.params = append(i.params, params)
	i.Unlock()
	time.Sleep(1 * time.Second)
	return nil
}

func TestWorkerPool(t *testing.T) {
	test.IntegrationTest(t) //required because a valid Kubeconfig is required to create test cluster entry

	//create cluster inventory
	inventory, err := cluster.NewInventory(db.NewTestConnection(t), true, &cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	//add cluster to inventory
	clusterState, err := inventory.CreateOrUpdate(1, kebTest.NewCluster(t, "1", 1, false, kebTest.OneComponentDummy))
	require.NoError(t, err)

	//cleanup created cluster
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
	}()

	//create test invoker to be able to verify invoker calls
	testInvoker := &testInvoker{errChannel: make(chan error, 10)}

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
	testInvoker.Add(1)
	require.NoError(t, workerPool.Run(ctx))
	require.WithinDuration(t, startTime, time.Now(), 3*time.Second) //ensure workerPool is considering ctx
	testInvoker.Wait()
	paramLock := sync.Mutex{}
	paramLock.Lock()
	params := testInvoker.params
	paramLock.Unlock()

	//verify that invoker was properly called
	require.Len(t, params, 1)
	require.Equal(t, clusterState, params[0].ClusterState)
	//Check cleanup params
	require.Equal(t, reconEntity.SchedulingID, params[0].SchedulingID)

	requireOpsProcessableExists(t, opsProcessable, params[0].CorrelationID)
}

func requireOpsProcessableExists(t *testing.T, opsProcessable []*model.OperationEntity, correlationID string) {
	for i := range opsProcessable {
		if opsProcessable[i].CorrelationID == correlationID {
			return
		}
	}
	t.Fatalf("Could not find correlationID: %s in opsProcessable list: %v", correlationID, opsProcessable)
}

func TestWorkerPoolParallel(t *testing.T) {

	t.Run("Multiple WorkerPools watching same reconciliation repository", func(t *testing.T) {

		var wg sync.WaitGroup

		const countWorkerPools = 5
		const countKebClusters = 3
		const countOperations = 3 //should be the same count as prio1 operations from all clusters; here three clusters with one prio1 operation each

		//prepare keb clusters
		kebClusters := []*keb.Cluster{
			kebTest.NewCluster(t, "1", 1, false, kebTest.OneComponentDummy),
			kebTest.NewCluster(t, "2", 1, false, kebTest.OneComponentDummy),
			kebTest.NewCluster(t, "3", 1, false, kebTest.OneComponentDummy),
		}

		//create mock database connection
		testDB := db.NewTestConnection(t)

		//create cluster inventory
		inventory, err := cluster.NewInventory(testDB, true, &cluster.MetricsCollectorMock{})
		require.NoError(t, err)

		//add clusters to inventory
		var clusterStates [countKebClusters]*cluster.State
		for i := range kebClusters {
			clusterStates[i], err = inventory.CreateOrUpdate(1, kebClusters[i])
		}
		require.NoError(t, err)

		//create test invoker to be able to verify invoker calls and update operation state
		testInvoker := &testInvokerParallel{errChannel: make(chan error, 100)}

		//create reconciliation for cluster
		testInvoker.reconRepo, err = reconciliation.NewPersistedReconciliationRepository(testDB, true)
		require.NoError(t, err)
		//cleanup before
		removeExistingReconciliations(t, map[string]reconciliation.Repository{"": testInvoker.reconRepo})

		for i := range clusterStates {
			_, err = testInvoker.reconRepo.CreateReconciliation(clusterStates[i], nil)
			require.NoError(t, err)
		}

		opsProcessable, err := testInvoker.reconRepo.GetProcessableOperations(0)
		require.Len(t, opsProcessable, countOperations) // only first priority
		require.NoError(t, err)

		//initialize worker pool
		wPools := make([]*Pool, countWorkerPools)
		for i := 0; i < countWorkerPools; i++ {
			workerPool, err := NewWorkerPool(&InventoryRetriever{inventory}, testInvoker.reconRepo, testInvoker, &Config{PoolSize: 5}, logger.NewLogger(true))
			require.NoError(t, err)
			wPools[i] = workerPool
		}

		//create time limited context
		ctx, cancelFct := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelFct()

		//start multiple worker pools
		errChannel := make(chan error)
		startAt := time.Now().Add(1 * time.Second)
		for i := 0; i < countWorkerPools; i++ {
			wg.Add(1)
			i := i
			go func(errChannel chan error, wPools []*Pool) {
				defer wg.Done()
				time.Sleep(time.Until(startAt))
				err := wPools[i].Run(ctx)
				if err != nil {
					errChannel <- err
				}
			}(errChannel, wPools)
		}
		wg.Wait()

		//check how often the invokes were successful
		require.Len(t, testInvoker.params, countOperations)
		for i := 0; i < len(testInvoker.params); i++ {
			//check for updated status and component
			require.Equal(t, model.ClusterStatusReconcilePending, testInvoker.params[i].ClusterState.Status.Status)
			require.Equal(t, model.CleanupComponent, testInvoker.params[i].ComponentToReconcile.Component)
		}
		//cleanup after
		removeExistingReconciliations(t, map[string]reconciliation.Repository{"": testInvoker.reconRepo})
	})
}

func removeExistingReconciliations(t *testing.T, repos map[string]reconciliation.Repository) {
	for _, repo := range repos {
		recons, err := repo.GetReconciliations(nil)
		require.NoError(t, err)
		for _, recon := range recons {
			require.NoError(t, repo.RemoveReconciliation(recon.SchedulingID))
		}
	}
}
