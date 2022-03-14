package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	kebTest "github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/pkg/errors"

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

	//create test invoker to be able to verify invoker calls
	testInvoker := &testInvoker{errChannel: make(chan error, 10)}

	//create reconciliation for cluster
	testInvoker.reconRepo = reconciliation.NewInMemoryReconciliationRepository()
	//cleanup created cluster
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
	}()

	reconEntity, err := testInvoker.reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
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

func TestWorkerPoolMaxOpRetriesReached(t *testing.T) {
	test.IntegrationTest(t) //required because a valid Kubeconfig is required to create test cluster entry

	//create mock database connection
	testDB := db.NewTestConnection(t)

	//create cluster inventory
	inventory, err := cluster.NewInventory(testDB, true, &cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	//add cluster to inventory
	clusterState, err := inventory.CreateOrUpdate(1, kebTest.NewCluster(t, "2", 1, false, kebTest.OneComponentDummy))
	require.NoError(t, err)

	//create test invoker to be able to verify invoker calls
	testInvoker := &testInvoker{}

	//create reconciliation for cluster
	testInvoker.reconRepo, err = reconciliation.NewPersistedReconciliationRepository(testDB, true)
	require.NoError(t, err)
	//cleanup created cluster
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
	}()

	reconEntity, err := testInvoker.reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
	require.NoError(t, err)
	defer func() { //cleanup at the end of the execution
		require.NoError(t, testInvoker.reconRepo.RemoveReconciliationBySchedulingID(reconEntity.SchedulingID))
	}()

	maxParallelOps := 25
	numberOfProcessableOps := 1
	opsProcessable, err := testInvoker.reconRepo.GetProcessableOperations(maxParallelOps)
	require.Len(t, opsProcessable, numberOfProcessableOps)
	require.NoError(t, err)

	//create worker pool config
	maxRetries := 1
	testConfig := &Config{MaxOperationRetries: maxRetries}

	//simulate one failed try from the component reconciler
	op := opsProcessable[0]
	retryID := uuid.NewString()
	err = testInvoker.reconRepo.UpdateOperationRetryID(op.SchedulingID, op.CorrelationID, retryID)
	require.NoError(t, err)

	//start worker pool
	workerPool, err := NewWorkerPool(&InventoryRetriever{inventory}, testInvoker.reconRepo, testInvoker, testConfig, logger.NewLogger(true))
	require.NoError(t, err)

	ctx, cancelFct := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelFct()

	//ensure worker pool stops when context gets closed
	startTime := time.Now()
	require.NoError(t, workerPool.Run(ctx))
	require.WithinDuration(t, startTime, time.Now(), 3*time.Second) //ensure workerPool is considering ctx

	//verify that invoker wasn't called
	invokedCnt := 0
	require.Len(t, testInvoker.params, invokedCnt)

	//get updated operation from the repository
	updatedOp, err := testInvoker.reconRepo.GetOperation(op.SchedulingID, op.CorrelationID)
	require.NoError(t, err)
	//verify operation is in error state with the correct reason
	require.Equal(t, model.OperationStateError, updatedOp.State)
	opErrReason := fmt.Sprintf("operation exceeds max. operation retries limit (maxOperationRetries:%d)", testConfig.MaxOperationRetries)
	require.Equal(t, opErrReason, updatedOp.Reason)

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
		testDB := db.NewTestConnection(t)

		inventory, err := cluster.NewInventory(testDB, true, &cluster.MetricsCollectorMock{})
		require.NoError(t, err)
		//create cluster inventory
		//create mock database connection
		defer func() {
			for _, kebCluster := range kebClusters {
				require.NoError(t, inventory.Delete(kebCluster.RuntimeID))
			}
		}()

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

		var recons []*model.ReconciliationEntity
		for i := range clusterStates {
			recon, err := testInvoker.reconRepo.CreateReconciliation(clusterStates[i], &model.ReconciliationSequenceConfig{})
			require.NoError(t, err)
			recons = append(recons, recon)
		}

		defer func() { //cleanup at the end of the test execution
			for _, recon := range recons {
				require.NoError(t, testInvoker.reconRepo.RemoveReconciliationBySchedulingID(recon.SchedulingID))
			}
		}()

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
			require.Equal(t, model.CRDComponent, testInvoker.params[i].ComponentToReconcile.Component)
		}
	})
}
