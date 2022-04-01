package service

import (
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func (s *serviceTestSuite) prepareTransitionTest(t *testing.T) (*ClusterStatusTransition, *cluster.State, func()) {
	dbConn, err := s.NewConnection()
	require.NoError(t, err)

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

	err = transition.StartReconciliation(clusterState.Cluster.RuntimeID, clusterState.Configuration.Version, &SchedulerConfig{
		PreComponents:  nil,
		DeleteStrategy: "",
	})
	require.NoError(t, err)

	//cleanup at the end of the execution
	cleanupFn := func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
		recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID})
		require.NoError(t, err)
		for _, recon := range recons {
			require.NoError(t, reconRepo.RemoveReconciliationBySchedulingID(recon.SchedulingID))
		}
		require.NoError(t, dbConn.Close())
	}
	return transition, clusterState, cleanupFn
}

func (s *serviceTestSuite) TestTransitionStartReconciliation() {
	t := s.T()
	transition, clusterState, cleanupFn := s.prepareTransitionTest(t)
	defer cleanupFn()

	oldClusterStateID := clusterState.Status.ID

	//starting reconciliation twice is not allowed
	err := transition.StartReconciliation(clusterState.Cluster.RuntimeID, clusterState.Configuration.Version, &SchedulerConfig{
		PreComponents:  nil,
		DeleteStrategy: "",
	})
	require.Error(t, err)

	//verify created reconciliation
	reconEntities, err := transition.reconRepo.GetReconciliations(
		&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID},
	)
	require.NoError(t, err)
	require.Len(t, reconEntities, 1)
	require.Greater(t, reconEntities[0].ClusterConfigStatus, oldClusterStateID) //verify new cluster-status ID is used
	require.False(t, reconEntities[0].Finished)

	//verify cluster status
	clusterState, err = transition.inventory.GetLatest(clusterState.Cluster.RuntimeID)
	require.NoError(t, err)
	require.Equal(t, clusterState.Status.Status, model.ClusterStatusReconciling)
}

func (s *serviceTestSuite) TestTransitionFinishReconciliation() {
	t := s.T()
	transition, clusterState, cleanupFn := s.prepareTransitionTest(t)
	defer cleanupFn()

	//get reconciliation entity
	reconEntities, err := transition.reconRepo.GetReconciliations(
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
	reconEntity, err := transition.reconRepo.GetReconciliation(reconEntities[0].SchedulingID)
	require.NoError(t, err)
	require.True(t, reconEntity.Finished)

	//verify cluster status
	clusterState, err = transition.inventory.GetLatest(clusterState.Cluster.RuntimeID)
	require.NoError(t, err)
	require.Equal(t, clusterState.Status.Status, model.ClusterStatusReady)
}

func (s *serviceTestSuite) TestTransitionFinishWhenClusterNotInProgress() {
	t := s.T()
	transition, clusterState, cleanupFn := s.prepareTransitionTest(t)
	defer cleanupFn()

	//get reconciliation entity
	reconEntities, err := transition.reconRepo.GetReconciliations(
		&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID},
	)
	require.NoError(t, err)

	err = transition.FinishReconciliation(reconEntities[0].SchedulingID, model.ClusterStatusReady)
	require.NoError(t, err)

	//get reconciliation entity
	reconEntity, err := transition.ReconciliationRepository().CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{
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
}

func (s *reconciliationTestSuite) TestDeletedClusters() {
	t := s.T()
	transition, clusterState, _ := s.prepareTransitionTest(t)
	toBeDeletedReconEntity, err := transition.reconRepo.CreateReconciliation(clusterState,
		&model.ReconciliationSequenceConfig{
			PreComponents:  [][]string{{"comp3"}},
			DeleteStrategy: "",
		})
	require.NoError(t, err)
	require.NotNil(t, toBeDeletedReconEntity)
	require.False(t, toBeDeletedReconEntity.Finished)

	// set cluster and related configs, statuses as deleted
	err = transition.inventory.Delete(clusterState.Cluster.RuntimeID)
	require.NoError(t, err)

	err = transition.CleanStatusesAndDeletedClustersOlderThan(time.Now())
	require.NoError(t, err)

	_, err = transition.inventory.Get(clusterState.Configuration.RuntimeID, clusterState.Configuration.Version)
	require.Error(t, err)
}
