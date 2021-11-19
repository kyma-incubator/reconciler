package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBookkeeper(t *testing.T) {
	dbConn := db.NewTestConnection(t) //share one db-connection between inventory and recon-repo (required for tx)

	//prepare inventory
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
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

	//mark all operations to be finished
	opEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
	require.NoError(t, err)
	for _, opEntity := range opEntities {
		err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, model.OperationStateDone)
		require.NoError(t, err)
	}

	//initialize bookkeeper
	bk := newBookkeeper(
		newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true)),
		&BookkeeperConfig{
			OperationsWatchInterval: 1 * time.Second,
			OrphanOperationTimeout:  2 * time.Second,
		},
		logger.NewLogger(true),
	)

	//run bookkeeper
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //stop bookkeeper after 5 sec
	defer cancel()

	start := time.Now()
	require.NoError(t, bk.Run(ctx))
	require.WithinDuration(t, time.Now(), start, 5500*time.Millisecond) //verify bookkeeper stops when ctx gets closed

	//verify bookkeeper results
	reconEntityUpdated, err := reconRepo.GetReconciliation(reconEntity.SchedulingID)
	require.NoError(t, err)
	require.True(t, reconEntityUpdated.Finished)

	//cleanup
	t.Log("Cleaning up test context")
	require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
	recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID})
	require.NoError(t, err)
	for _, recon := range recons {
		require.NoError(t, reconRepo.RemoveReconciliation(recon.SchedulingID))
	}
}

func TestBookkeeperParallel(t *testing.T) {
	tests := []struct {
		name        string
		markOpsDone bool
		customFunc  string
		errMessage  string
		errCount    int
	}{
		{name: "Mark two operations as orphan in multiple parallel threads", markOpsDone: false, customFunc: "markOrphanOperations", errMessage: "Bookkeeper failed to update status of orphan operation", errCount: 98},
		{name: "Finish two operations in multiple parallel threads", markOpsDone: true, customFunc: "finishReconciliation", errMessage: "Bookkeeper failed to update cluster", errCount: 49},
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

			if tc.markOpsDone {
				//mark all operations to be done
				opEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
				require.NoError(t, err)
				for _, opEntity := range opEntities {
					err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, model.OperationStateDone)
					require.NoError(t, err)
				}
			}

			//initialize logger including error channel
			errChannel := make(chan error, 100)
			bookkeeperLogger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.Hooks(func(e zapcore.Entry) error {
				if strings.Contains(e.Message, tc.errMessage) {
					errChannel <- errors.New("Update failed")
				}
				return nil
			})))

			//initialize bookkeeper
			bk := newBookkeeper(
				newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true)),
				&BookkeeperConfig{
					OperationsWatchInterval: 100 * time.Millisecond,
					OrphanOperationTimeout:  5 * time.Second,
				},
				bookkeeperLogger.Sugar(),
			)

			//setup reconciliation result
			recons, err := reconRepo.GetReconciliations(nil)
			require.NoError(t, err)
			reconResult, err := bk.newReconciliationResult(recons[0])
			require.NoError(t, err)
			reconResult.orphanTimeout = 0 * time.Microsecond

			//call markOrphanOperations in parallel threads
			startAt := time.Now().Add(2 * time.Second)
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					switch tc.customFunc {
					case "markOrphanOperations":
						time.Sleep(time.Until(startAt))
						bk.markOrphanOperations(reconResult)
					case "finishReconciliation":
						time.Sleep(time.Until(startAt))
						bk.finishReconciliation(reconResult)
					default:
						t.Errorf("Unknown function: %s", tc.customFunc)
					}
				}()
			}
			time.Sleep(5 * time.Second)

			require.Equal(t, tc.errCount, len(errChannel))
		})
	}
}
