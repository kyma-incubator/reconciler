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

func TestBookkeepingtaskParallel(t *testing.T) {
	tests := []struct {
		name        string
		markOpsDone bool
		customFunc  string
		errMessage  string
		errCount    int
		task        string
	}{
		{name: "Mark two operations as orphan in multiple parallel threads", markOpsDone: false, customFunc: "markOrphanOperations", errMessage: "Bookkeeper failed to update status of orphan operation", errCount: 48, task: "markOrphanOperation"},
		{name: "Finish two operations in multiple parallel threads", markOpsDone: true, customFunc: "finishReconciliation", errMessage: "Bookkeeper failed to update cluster", errCount: 24, task: "finishOperation"},
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
			var bookkeeperOperation BookkeepingTask
			switch tc.task {
			case "markOrphanOperation":
				bookkeeperOperation = markOrphanOperation{
					transition: transition,
					logger:     logger.NewLogger(true),
				}
			case "finishOperation":
				bookkeeperOperation = finishOperation{
					transition: transition,
					logger:     logger.NewLogger(true),
				}
			default:
				t.Errorf("Unknown task: %s", tc.task)
			}

			//initialize bookkeeper
			bk := newBookkeeper(
				reconRepo,
				&BookkeeperConfig{
					OperationsWatchInterval: 100 * time.Millisecond,
					OrphanOperationTimeout:  5 * time.Second,
					MaxRetries:              150,
				},
				logger.NewLogger(true),
			)

			//setup reconciliation result
			recons, err := reconRepo.GetReconciliations(nil)
			require.NoError(t, err)
			reconResult, err := bk.newReconciliationResult(recons[0])
			require.NoError(t, err)
			reconResult.orphanTimeout = 0 * time.Microsecond

			//call Apply in parallel threads
			errChannel := make(chan error, 100)
			startAt := time.Now().Add(2 * time.Second)
			for i := 0; i < 25; i++ {
				wg.Add(1)
				go func(errChannel chan error, bookkeeperOperation BookkeepingTask) {
					defer wg.Done()
					time.Sleep(time.Until(startAt))
					err := bookkeeperOperation.Apply(reconResult, bk.config.MaxRetries)
					for _, e := range err {
						errChannel <- e
					}
				}(errChannel, bookkeeperOperation)
			}
			wg.Wait()

			require.Equal(t, tc.errCount, len(errChannel))
		})
	}
}
