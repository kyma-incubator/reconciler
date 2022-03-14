package service

import (
	"sync"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
	"github.com/stretchr/testify/require"
)

func TestBookkeepingTask(t *testing.T) {
	tests := []struct {
		name           string
		markOpsAs      model.OperationState
		customFunc     func(transition *ClusterStatusTransition) BookkeepingTask
		expectedStatus model.OperationState
	}{
		{name: "Mark operations as orphan", markOpsAs: model.OperationStateInProgress, customFunc: func(transition *ClusterStatusTransition) BookkeepingTask {
			return markOrphanOperation{
				transition: transition,
				logger:     logger.NewLogger(true),
			}
		}, expectedStatus: model.OperationStateOrphan},
		{name: "Finish operations", markOpsAs: model.OperationStateDone, customFunc: func(transition *ClusterStatusTransition) BookkeepingTask {
			return finishOperation{
				transition: transition,
				logger:     logger.NewLogger(true),
			}
		}, expectedStatus: model.OperationStateDone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			//create mock database connection
			dbConn := db.NewTestConnection(t)
			//prepare inventory
			inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
			require.NoError(t, err)

			testCluster := test.NewCluster(t, "random", 1, false, test.OneComponentDummy)
			defer func() {
				require.NoError(t, inventory.Delete(testCluster.RuntimeID))
			}()

			//add cluster to inventory
			clusterState, err := inventory.CreateOrUpdate(1, testCluster)
			require.NoError(t, err)

			//trigger reconciliation for cluster
			reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
			require.NoError(t, err)

			reconEntity := newReconciliation(t, reconRepo, clusterState)
			defer func() {
				removeExistingReconciliations(t, reconRepo)
			}()

			//mark all operations to a specific state, if needed for tc
			if tc.markOpsAs != "" {
				opEntities, err := reconRepo.GetOperations(&operation.WithSchedulingID{
					SchedulingID: reconEntity.SchedulingID,
				})
				require.NoError(t, err)
				for _, opEntity := range opEntities {
					err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, tc.markOpsAs, true)
					require.NoError(t, err)
				}
			}

			//setup bookkeeper task
			transition := newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true))

			//initialize bookkeeper
			bk := newBookkeeper(
				reconRepo,
				&BookkeeperConfig{
					OperationsWatchInterval: 100 * time.Millisecond,
					OrphanOperationTimeout:  1 * time.Microsecond,
					MaxReconcileErrRetries:  150,
				},
				logger.NewLogger(true),
			)

			//setup reconciliation result
			reconResult := getReconResult(t, reconRepo, bk)

			//execute bookkeepingtask
			errSlice := tc.customFunc(transition).Apply(reconResult, bk.config)
			for _, e := range errSlice {
				require.NoError(t, e)
			}

			//check correct status
			reconResult = getReconResult(t, reconRepo, bk)
			operations := reconResult.GetOperations()
			for _, o := range operations {
				require.Equal(t, tc.expectedStatus, o.State)
			}
		})
	}
}

func newReconciliation(t *testing.T, reconRepo reconciliation.Repository, clusterState *cluster.State) *model.ReconciliationEntity {
	var reconEntity *model.ReconciliationEntity
	recons, err := reconRepo.GetReconciliations(&reconciliation.CurrentlyReconcilingWithRuntimeID{
		RuntimeID: clusterState.Cluster.RuntimeID,
	})
	require.NoError(t, err)
	if len(recons) == 0 {
		reconEntity, err = reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
		require.NoError(t, err)
	} else {
		require.Len(t, recons, 1)
		reconEntity = recons[0]
	}
	require.NotEmpty(t, reconEntity.Lock)
	require.False(t, reconEntity.Finished)
	return reconEntity
}

func TestBookkeepingTaskParallel(t *testing.T) {

	threadCount := 25
	//errorCount should be equal to 48, since there are 2 operations scheduled,
	//and 25 threads try to mark them as orphans at the same time
	//three should succeed, resolving in (2*25)-2=48 errors
	orphanErrors := 48
	//finishErrors should be equal to 24, since 25 threads try to update the cluster state to be finished
	//only one should succeed resolving in 24 errors
	finishErrors := 24

	tests := []struct {
		name           string
		markOpsAs      model.OperationState
		customFunc     func(transition *ClusterStatusTransition) BookkeepingTask
		errMessage     string
		errCount       int
		expectedStatus model.OperationState
	}{
		{name: "Mark two operations as orphan in multiple parallel threads", markOpsAs: model.OperationStateInProgress, customFunc: func(transition *ClusterStatusTransition) BookkeepingTask {
			return markOrphanOperation{
				transition: transition,
				logger:     logger.NewLogger(true),
			}
		}, errMessage: "Bookkeeper failed to update status of orphan operation", errCount: orphanErrors, expectedStatus: model.OperationStateOrphan},
		{name: "Finish two operations in multiple parallel threads", markOpsAs: model.OperationStateDone, customFunc: func(transition *ClusterStatusTransition) BookkeepingTask {
			return finishOperation{
				transition: transition,
				logger:     logger.NewLogger(true),
			}
		}, errMessage: "Bookkeeper failed to update cluster", errCount: finishErrors, expectedStatus: model.OperationStateDone},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			//initialize WaitGroup
			var wg sync.WaitGroup

			//create mock database connection
			dbConn := db.NewTestConnection(t)
			//prepare inventory
			inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
			require.NoError(t, err)

			//add cluster to inventory
			clusterState, err := inventory.CreateOrUpdate(1, test.NewCluster(t, "random", 1, false, test.OneComponentDummy))
			require.NoError(t, err)
			//cleanup cluster at the end
			defer func() {
				require.NoError(t, inventory.Delete(clusterState.Status.RuntimeID))
			}()

			//cleanup cluster at the end
			defer func() {
				require.NoError(t, inventory.Delete(clusterState.Status.RuntimeID))
			}()

			//trigger reconciliation for cluster
			reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
			require.NoError(t, err)
			//cleanup before
			removeExistingReconciliations(t, reconRepo)

			reconEntity, err := reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})

			require.NoError(t, err)
			require.NotEmpty(t, reconEntity.Lock)
			require.False(t, reconEntity.Finished)

			//mark all operations to wanted state
			opEntities, err := reconRepo.GetOperations(&operation.WithSchedulingID{
				SchedulingID: reconEntity.SchedulingID,
			})
			require.NoError(t, err)
			for _, opEntity := range opEntities {
				err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, tc.markOpsAs, true)
				require.NoError(t, err)
			}

			//setup bookkeeper task
			transition := newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true))

			//initialize bookkeeper
			bk := newBookkeeper(
				reconRepo,
				&BookkeeperConfig{
					OperationsWatchInterval: 100 * time.Millisecond,
					OrphanOperationTimeout:  1 * time.Microsecond,
					MaxReconcileErrRetries:  150,
				},
				logger.NewLogger(true),
			)

			//setup reconciliation result
			reconResult := getReconResult(t, reconRepo, bk)

			//call Apply in parallel threads
			errChannel := make(chan error, 100)
			startAt := time.Now().Add(2 * time.Second)
			for i := 0; i < threadCount; i++ {
				wg.Add(1)
				go func(errChannel chan error, bookkeeperOperation BookkeepingTask) {
					defer wg.Done()
					time.Sleep(time.Until(startAt))
					err := bookkeeperOperation.Apply(reconResult, bk.config)
					for _, e := range err {
						errChannel <- e
					}
				}(errChannel, tc.customFunc(transition))
			}
			wg.Wait()

			//check failed amount bookkeepingtasks
			require.Equal(t, tc.errCount, len(errChannel))
			//check correct status
			reconResult = getReconResult(t, reconRepo, bk)
			operations := reconResult.GetOperations()
			for _, o := range operations {
				require.Equal(t, tc.expectedStatus, o.State)
			}
			//cleanup after
			removeExistingReconciliations(t, reconRepo)
		})
	}
}

func getReconResult(t *testing.T, reconRepo reconciliation.Repository, bk *bookkeeper) *ReconciliationResult {
	recons, err := reconRepo.GetReconciliations(nil)
	require.NoError(t, err)
	reconResult, err := bk.newReconciliationResult(recons[0])
	require.NoError(t, err)
	return reconResult
}

func removeExistingReconciliations(t *testing.T, repo reconciliation.Repository) {
	recons, err := repo.GetReconciliations(nil)
	require.NoError(t, err)
	for _, recon := range recons {
		require.NoError(t, repo.RemoveReconciliation(recon.SchedulingID))
	}
}
