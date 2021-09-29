package service

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTransition(t *testing.T) {
	dbConn := db.NewTestConnection(t)

	//create inventory and test cluster entry
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: "123",
		KymaConfig: keb.KymaConfig{
			Components: nil,
			Profile:    "",
			Version:    "1.2.3",
		},
		RuntimeID: "testCluster",
	})
	require.NoError(t, err)

	//create reconciliation entity for the cluster
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)

	//create transition which will change cluster states
	transition := newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true))

	t.Run("Start Reconciliation", func(t *testing.T) {
		err := transition.StartReconciliation(clusterState, nil)
		require.NoError(t, err)

		//starting reconciliation twice is not allowed
		err = transition.StartReconciliation(clusterState, nil)
		require.Error(t, err)

		//verify created reconciliation
		reconEntities, err := reconRepo.GetReconciliations(
			&reconciliation.WithCluster{Cluster: clusterState.Cluster.Cluster},
		)
		require.NoError(t, err)
		require.Len(t, reconEntities, 1)
		require.True(t, reconEntities[0].IsReconciling())

		//verify cluster status
		clusterState, err := inventory.GetLatest(clusterState.Cluster.Cluster)
		require.NoError(t, err)
		require.Equal(t, clusterState.Status.Status, model.ClusterStatusReconciling)
	})

	t.Run("Finish Reconciliation", func(t *testing.T) {
		//get reconciliation entity
		reconEntities, err := reconRepo.GetReconciliations(
			&reconciliation.WithCluster{Cluster: clusterState.Cluster.Cluster},
		)
		require.NoError(t, err)
		require.Len(t, reconEntities, 1)
		require.True(t, reconEntities[0].IsReconciling())

		//finish the reconciliation
		err = transition.FinishReconciliation(reconEntities[0].SchedulingID, model.ClusterStatusReady)
		require.NoError(t, err)

		//finishing reconciliation twice is not allowed
		err = transition.FinishReconciliation(reconEntities[0].SchedulingID, model.ClusterStatusReady)
		require.Error(t, err)

		//verify that reconciliation is finished
		reconEntity, err := reconRepo.GetReconciliation(reconEntities[0].SchedulingID)
		require.NoError(t, err)
		require.False(t, reconEntity.IsReconciling())

		//verify cluster status
		clusterState, err := inventory.GetLatest(clusterState.Cluster.Cluster)
		require.NoError(t, err)
		require.Equal(t, clusterState.Status.Status, model.ClusterStatusReady)
	})

}
