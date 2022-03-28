package service

import (
	"testing"
	"time"

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

	testSchedulerConfig := &SchedulerConfig{
		PreComponents:  nil,
		DeleteStrategy: "",
	}

	//cleanup at the end of the execution
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
		recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID})
		require.NoError(t, err)
		for _, recon := range recons {
			require.NoError(t, reconRepo.RemoveReconciliationBySchedulingID(recon.SchedulingID))
		}
	}()

	t.Run("Start Reconciliation", func(t *testing.T) {
		oldClusterStateID := clusterState.Status.ID
		err := transition.StartReconciliation(clusterState.Cluster.RuntimeID, clusterState.Configuration.Version, testSchedulerConfig)
		require.NoError(t, err)

		//starting reconciliation twice is not allowed
		err = transition.StartReconciliation(clusterState.Cluster.RuntimeID, clusterState.Configuration.Version, testSchedulerConfig)
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

	t.Run("Finish Reconciliation When Cluster is not in progress", func(t *testing.T) {
		//get reconciliation entity
		reconEntity, err := reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{
			PreComponents:  nil,
			DeleteStrategy: "",
		})
		require.NoError(t, err)
		require.NotNil(t, reconEntity)
		require.False(t, reconEntity.Finished)

		//retrieving cluster state
		currentClusterState, err := transition.inventory.GetLatest(clusterState.Cluster.RuntimeID)
		require.NoError(t, err)

		//setting cluster state manually
		_, err = transition.inventory.UpdateStatus(currentClusterState, model.ClusterStatusDeletePending)
		require.NoError(t, err)

		//verify reconciliation success
		err = transition.FinishReconciliation(reconEntity.SchedulingID, model.ClusterStatusReady)
		require.NoError(t, err)

		//verify cluster status
		newClusterState, err := transition.inventory.GetLatest(clusterState.Cluster.RuntimeID)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusDeletePending, newClusterState.Status.Status)
	})

	t.Run("Clean Deleted Clusters", func(t *testing.T) {
		toBeDeletedClusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
			Kubeconfig: test.ReadKubeconfig(t),
			KymaConfig: keb.KymaConfig{
				Components: []keb.Component{
					{
						Component: "TestComp2",
					},
				},
				Profile: "",
				Version: "1.2.3",
			},
			RuntimeID: uuid.NewString(),
		})
		require.NoError(t, err)
		toBeDeletedReconEntity, err := reconRepo.CreateReconciliation(toBeDeletedClusterState,
			&model.ReconciliationSequenceConfig{
				PreComponents:  [][]string{{"comp3"}},
				DeleteStrategy: "",
			})
		require.NoError(t, err)
		require.NotNil(t, toBeDeletedReconEntity)
		require.False(t, toBeDeletedReconEntity.Finished)

		err = inventory.Delete(toBeDeletedClusterState.Cluster.RuntimeID)
		require.NoError(t, err)

		err = transition.CleanDeletedClusters(time.Now())
		require.NoError(t, err)

		clusterStates, err := inventory.GetAll()
		require.NoError(t, err)
		require.Equal(t, 1, len(clusterStates))

	})

}
