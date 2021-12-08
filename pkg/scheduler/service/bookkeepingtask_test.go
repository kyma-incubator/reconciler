package service

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/stretchr/testify/require"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestBookkeepingtask(t *testing.T) {
	tests := []struct {
		name           string
		markOpsDone    bool
		customFunc     func(transition *ClusterStatusTransition) BookkeepingTask
		expectedStatus model.OperationState
	}{
		{name: "Mark operations as orphan", markOpsDone: false, customFunc: func(transition *ClusterStatusTransition) BookkeepingTask {
			return markOrphanOperation{
				transition: transition,
				logger:     logger.NewLogger(true),
			}
		}, expectedStatus: model.OperationStateOrphan},
		{name: "Finish operations", markOpsDone: true, customFunc: func(transition *ClusterStatusTransition) BookkeepingTask {
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

			//add cluster to inventory
			clusterState, err := inventory.CreateOrUpdate(1, test.NewCluster(t, strconv.Itoa(1), 1, false, test.OneComponentDummy))
			require.NoError(t, err)

			//trigger reconciliation for cluster
			reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
			require.NoError(t, err)
			reconEntity, err := reconRepo.CreateReconciliation(clusterState, nil)
			require.NoError(t, err)
			require.NotEmpty(t, reconEntity.Lock)
			require.False(t, reconEntity.Finished)

			//mark all operations to be done, if needed for tc
			if tc.markOpsDone {
				opEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
				require.NoError(t, err)
				for _, opEntity := range opEntities {
					err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, model.OperationStateDone, true)
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
					MaxRetries:              150,
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

func TestBookkeepingtaskParallel(t *testing.T) {
	tests := []struct {
		name           string
		markOpsDone    bool
		customFunc     func(transition *ClusterStatusTransition) BookkeepingTask
		errMessage     string
		errCount       int
		expectedStatus model.OperationState
	}{
		{name: "Mark two operations as orphan in multiple parallel threads", markOpsDone: false, customFunc: func(transition *ClusterStatusTransition) BookkeepingTask {
			return markOrphanOperation{
				transition: transition,
				logger:     logger.NewLogger(true),
			}
		}, errMessage: "Bookkeeper failed to update status of orphan operation", errCount: 48, expectedStatus: model.OperationStateOrphan},
		{name: "Finish two operations in multiple parallel threads", markOpsDone: true, customFunc: func(transition *ClusterStatusTransition) BookkeepingTask {
			return finishOperation{
				transition: transition,
				logger:     logger.NewLogger(true),
			}
		}, errMessage: "Bookkeeper failed to update cluster", errCount: 24, expectedStatus: model.OperationStateDone},
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
			clusterState, err := inventory.CreateOrUpdate(1, test.NewCluster(t, strconv.Itoa(1), 1, false, test.OneComponentDummy))
			require.NoError(t, err)

			//trigger reconciliation for cluster
			reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
			require.NoError(t, err)
			reconEntity, err := reconRepo.CreateReconciliation(clusterState, nil)
			require.NoError(t, err)
			require.NotEmpty(t, reconEntity.Lock)
			require.False(t, reconEntity.Finished)

			//mark all operations to be done, if needed for tc
			if tc.markOpsDone {
				opEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
				require.NoError(t, err)
				for _, opEntity := range opEntities {
					err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, model.OperationStateDone, true)
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
					MaxRetries:              150,
				},
				logger.NewLogger(true),
			)

			//setup reconciliation result
			reconResult := getReconResult(t, reconRepo, bk)

			//call Apply in parallel threads
			errChannel := make(chan error, 100)
			startAt := time.Now().Add(2 * time.Second)
			for i := 0; i < 25; i++ {
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
