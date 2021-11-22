package service

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBookkeepingtaskParallel(t *testing.T) {
	tests := []struct {
		name        string
		markOpsDone bool
		customFunc  string
		errMessage  string
		errCount    uint64
		task string
	}{
		{name: "Mark two operations as orphan in multiple parallel threads", markOpsDone: false, customFunc: "markOrphanOperations", errMessage: "Bookkeeper failed to update status of orphan operation", errCount: 98, task: "orphanOperation"},
		{name: "Finish two operations in multiple parallel threads", markOpsDone: true, customFunc: "finishReconciliation", errMessage: "Bookkeeper failed to update cluster", errCount: 49, task: "finishOperation"},
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
			clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
				Kubeconfig: "123",
				KymaConfig: keb.KymaConfig{
					Components: []keb.Component{
						{
							Component:     "dummy",
							Configuration: nil,
							Namespace:     "kyma-system",
						},
					},
					Profile: "",
					Version: "1.2.3",
				},
				RuntimeID: uuid.NewString(),
			})
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
					err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, model.OperationStateDone)
					require.NoError(t, err)
				}
			}

			//setup bookkeeper task
			transition := newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true))
			var bookkeeperOperation BookkeepingTask
			switch tc.task {
			case "orphanOperation":
				bookkeeperOperation = orphanOperation{
					transition: transition,
					logger: logger.NewLogger(true),
				}
			case "finishOperation":
				bookkeeperOperation = finishOperation{
					transition: transition,
					logger: logger.NewLogger(true),
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
			var errCnt uint64 = 0
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func(errChannel chan error, bookkeeperOperation BookkeepingTask) {
					defer wg.Done()
					time.Sleep(time.Until(startAt))
					err, cnt := bookkeeperOperation.Apply(reconResult)
					if err != nil {
						fmt.Printf("Error: %s\n",err)
						errChannel <- err
						atomic.AddUint64(&errCnt, uint64(cnt))
						fmt.Printf("Counter: %d\n", errCnt)
					}
				}(errChannel, bookkeeperOperation)
			}
			wg.Wait()

			require.Equal(t, tc.errCount, errCnt)
		})
	}
}
