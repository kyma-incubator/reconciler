package service

import (
	"context"
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

var clusterState *cluster.State

func TestScheduler(t *testing.T) {
	clusterState = &cluster.State{
		Cluster: &model.ClusterEntity{
			Version:    1,
			RuntimeID:  "testCluster",
			Kubeconfig: "xyz",
			Contract:   1,
		},
		Configuration: &model.ClusterConfigurationEntity{
			Version:        1,
			RuntimeID:      "testCluster",
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
			ID:             1,
			RuntimeID:      "testCluster",
			ClusterVersion: 1,
			ConfigVersion:  1,
			Status:         model.ClusterStatusReconcilePending,
		},
	}

	t.Run("Test run once", func(t *testing.T) {
		reconRepo := reconciliation.NewInMemoryReconciliationRepository()
		scheduler := newScheduler(nil, logger.NewLogger(true))
		require.NoError(t, scheduler.RunOnce(clusterState, reconRepo))
		requirecReconciliationEntity(t, reconRepo)
	})

	t.Run("Test run", func(t *testing.T) {
		reconRepo := reconciliation.NewInMemoryReconciliationRepository()
		scheduler := newScheduler(nil, logger.NewLogger(true))

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()

		err := scheduler.Run(ctx, &ClusterStatusTransition{
			conn: db.NewTestConnection(t),
			inventory: &cluster.MockInventory{
				//this will cause the creation of a reconciliation for the same cluster multiple times:
				ClustersToReconcileResult: []*cluster.State{
					clusterState,
				},
				//simulate an updated cluster status (required when transition updates the cluster status)
				UpdateStatusResult: func() *cluster.State {
					clusterState.Status.Status = model.ClusterStatusReconciling
					return clusterState
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
		requirecReconciliationEntity(t, reconRepo)
	})
}

func requirecReconciliationEntity(t *testing.T, reconRepo reconciliation.Repository) {
	recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: "testCluster"})
	require.NoError(t, err)
	require.Len(t, recons, 1)
	require.Equal(t, recons[0].RuntimeID, clusterState.Cluster.RuntimeID)
	ops, err := reconRepo.GetOperations(recons[0].SchedulingID)
	require.NoError(t, err)
	require.Len(t, ops, 2)
	require.Equal(t, ops[0].RuntimeID, clusterState.Cluster.RuntimeID)
}
