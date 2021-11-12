package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/stretchr/testify/require"
)


var (
	dbConn db.Connection
	mu     sync.Mutex
)

func dbConnection(t *testing.T) db.Connection {
	mu.Lock()
	defer mu.Unlock()
	if dbConn == nil {
		dbConn = db.NewTestConnection(t)
	}
	return dbConn
}

func TestSchedulerParallel(t *testing.T) {
	t.Run("", func(t *testing.T) {

		scheduler := newScheduler(nil, logger.NewLogger(true))
		reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConnection(t), true)
		require.NoError(t, err)
		inventory :=  &cluster.MockInventory{
			ClustersToReconcileResult: []*cluster.State{
				testClusterState("testClusterA", 1),
				testClusterState("testClusterB", 2),
				testClusterState("testClusterC", 3),
			},
			UpdateStatusResult: func() *cluster.State {
				updatedState := testClusterState("testClusterA", 1)
				updatedState.Status.Status = model.ClusterStatusReconciling
				return updatedState
			}(),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		startAt := time.Now().Add(2 * time.Second)
		for i := 0; i < 20; i++ {
			go func() {
				time.Sleep(startAt.Sub(time.Now()))
				err := scheduler.Run(ctx, &ClusterStatusTransition{
					conn: dbConnection(t),
					inventory: inventory,
					reconRepo: reconRepo,
					logger:    logger.NewLogger(true),
				}, &SchedulerConfig{
					InventoryWatchInterval:   100 * time.Millisecond,
					ClusterReconcileInterval: 100 * time.Second,
					ClusterQueueSize:         10,
				})
				require.NoError(t, err)
			}()
		}
		time.Sleep(3 *time.Second)

		recons, err := reconRepo.GetReconciliations(nil)
		require.NoError(t, err)
		require.Equal(t, 3, len(recons))
	})
}

func TestScheduler(t *testing.T) {
	t.Run("Test run once", func(t *testing.T) {
		clusterState := testClusterState("testCluster", 1)
		reconRepo := reconciliation.NewInMemoryReconciliationRepository()
		scheduler := newScheduler(nil, logger.NewLogger(true))
		require.NoError(t, scheduler.RunOnce(clusterState, reconRepo))
		requirecReconciliationEntity(t, reconRepo, clusterState)
	})

	t.Run("Test run", func(t *testing.T) {
		reconRepo := reconciliation.NewInMemoryReconciliationRepository()
		scheduler := newScheduler(nil, logger.NewLogger(true))

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()

		clusterState := testClusterState("testCluster",1)

		err := scheduler.Run(ctx, &ClusterStatusTransition{
			conn: db.NewTestConnection(t),
			inventory: &cluster.MockInventory{
				//this will cause the creation of a reconciliation for the same cluster multiple times:
				ClustersToReconcileResult: []*cluster.State{
					clusterState,
				},
				//simulate an updated cluster status (required when transition updates the cluster status)
				UpdateStatusResult: func() *cluster.State {
					updatedState := testClusterState("testCluster",1 )
					updatedState.Status.Status = model.ClusterStatusReconciling
					return updatedState
				}(),
			},
			reconRepo: reconRepo,
			logger:    logger.NewLogger(true),
		}, &SchedulerConfig{
			InventoryWatchInterval:   250 * time.Millisecond,
			ClusterReconcileInterval: 100 * time.Second,
			ClusterQueueSize:         5,
		})
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond) //give it some time to shutdown

		require.WithinDuration(t, start, time.Now(), 2*time.Second)
		requirecReconciliationEntity(t, reconRepo, clusterState)
	})
}

func requirecReconciliationEntity(t *testing.T, reconRepo reconciliation.Repository, state *cluster.State) {
	recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: "testCluster"})
	require.NoError(t, err)
	require.Len(t, recons, 1)
	require.Equal(t, recons[0].RuntimeID, state.Cluster.RuntimeID)
	ops, err := reconRepo.GetOperations(recons[0].SchedulingID)
	require.NoError(t, err)
	require.Len(t, ops, 2)
	require.Equal(t, ops[0].RuntimeID, state.Cluster.RuntimeID)
}

func testClusterState(clusterID string, statusID int64) *cluster.State {
	return &cluster.State{
		Cluster: &model.ClusterEntity{
			Version:    1,
			RuntimeID:  clusterID,
			Kubeconfig: "xyz",
			Contract:   1,
		},
		Configuration: &model.ClusterConfigurationEntity{
			Version:        1,
			RuntimeID:      clusterID,
			ClusterVersion: 1,
			Contract:       1,
			KymaVersion:    "1.24.0",
			Components: []*keb.Component{
				{
					Component: "testComp1",
					Version:   "1",
				},
			},
		},
		Status: &model.ClusterStatusEntity{
			ID:             statusID,
			RuntimeID:      clusterID,
			ClusterVersion: 1,
			ConfigVersion:  1,
			Status:         model.ClusterStatusReconcilePending,
		},
	}
}
