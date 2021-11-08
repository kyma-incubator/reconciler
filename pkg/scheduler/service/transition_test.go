package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestTransition(t *testing.T) {
	test.IntegrationTest(t)

	dbConn := db.NewTestConnection(t)

	//create inventory and test cluster entry
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Components: []keb.Component{
				{
					Component: "TestComp1",
				},
			},
			Profile: "",
			Version: "1.2.3",
		},
		RuntimeID: uuid.NewString(),
	})
	require.NoError(t, err)

	//create reconciliation entity for the cluster
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)

	//create transition which will change cluster states
	transition := newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true))

	//cleanup at the end of the execution
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
		recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID})
		require.NoError(t, err)
		for _, recon := range recons {
			require.NoError(t, reconRepo.RemoveReconciliation(recon.SchedulingID))
		}
	}()

	t.Run("Start Reconciliation", func(t *testing.T) {
		oldClusterStateID := clusterState.Status.ID
		err := transition.StartReconciliation(clusterState, nil)
		require.NoError(t, err)

		//starting reconciliation twice is not allowed
		err = transition.StartReconciliation(clusterState, nil)
		require.Error(t, err)

		//verify created reconciliation
		reconEntities, err := reconRepo.GetReconciliations(
			&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID},
		)
		require.NoError(t, err)
		require.Len(t, reconEntities, 1)
		require.Greater(t, reconEntities[0].ClusterConfigStatus, oldClusterStateID) //verify new cluster-status ID is used
		require.False(t, reconEntities[0].Finished)

		//verify cluster status
		clusterState, err := inventory.GetLatest(clusterState.Cluster.RuntimeID)
		require.NoError(t, err)
		require.Equal(t, clusterState.Status.Status, model.ClusterStatusReconciling)
	})

	t.Run("Finish Reconciliation", func(t *testing.T) {
		//get reconciliation entity
		reconEntities, err := reconRepo.GetReconciliations(
			&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID},
		)
		require.NoError(t, err)
		require.Len(t, reconEntities, 1)
		require.False(t, reconEntities[0].Finished)

		//finish the reconciliation
		err = transition.FinishReconciliation(reconEntities[0].SchedulingID, model.ClusterStatusReady)
		require.NoError(t, err)

		//finishing reconciliation twice is not allowed
		err = transition.FinishReconciliation(reconEntities[0].SchedulingID, model.ClusterStatusReady)
		require.Error(t, err)

		//verify that reconciliation is finished
		reconEntity, err := reconRepo.GetReconciliation(reconEntities[0].SchedulingID)
		require.NoError(t, err)
		require.True(t, reconEntity.Finished)

		//verify cluster status
		clusterState, err := inventory.GetLatest(clusterState.Cluster.RuntimeID)
		require.NoError(t, err)
		require.Equal(t, clusterState.Status.Status, model.ClusterStatusReady)
	})

}
