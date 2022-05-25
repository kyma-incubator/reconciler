package service

import (
	"fmt"
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

func (s *serviceTestSuite) prepareTransitionTest(t *testing.T, count int) []*cluster.State {
	var err error

	s.dbConn, err = s.NewConnection()
	require.NoError(t, err)

	//create inventory and test cluster entry
	s.inventory, err = cluster.NewInventory(s.dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	//create reconciliation entity for the cluster
	s.reconRepo, err = reconciliation.NewPersistedReconciliationRepository(s.dbConn, true)
	require.NoError(t, err)

	//create transition which will change cluster states
	s.transition = newClusterStatusTransition(s.dbConn, s.inventory, s.reconRepo, logger.NewLogger(true))

	clusterStates := make([]*cluster.State, 0, count)
	for i := 0; i < count; i++ {
		clusterState, err := s.inventory.CreateOrUpdate(1, &keb.Cluster{
			Kubeconfig: test.ReadKubeconfig(t),
			KymaConfig: keb.KymaConfig{
				Components: []keb.Component{
					{
						Component: fmt.Sprintf("TestComp%d", i),
					},
				},
				Profile: "",
				Version: fmt.Sprintf("1.2.3.%d", i),
			},
			RuntimeID: uuid.NewString(),
		})
		require.NoError(t, err)
		clusterStates = append(clusterStates, clusterState)

		err = s.transition.StartReconciliation(clusterState.Cluster.RuntimeID, clusterState.Configuration.Version, &SchedulerConfig{
			PreComponents:  nil,
			DeleteStrategy: "",
		})
		require.NoError(t, err)

		s.runtimeIDsToClear = []string{clusterState.Cluster.RuntimeID}
	}
	return clusterStates
}

func (s *serviceTestSuite) TestTransitionStartReconciliation() {
	t := s.T()
	clusterStates := s.prepareTransitionTest(t, 1)

	oldClusterStateID := clusterStates[0].Status.ID

	//starting reconciliation twice is not allowed
	err := s.transition.StartReconciliation(clusterStates[0].Cluster.RuntimeID, clusterStates[0].Configuration.Version, &SchedulerConfig{
		PreComponents:  nil,
		DeleteStrategy: "",
	})
	require.Error(t, err)

	//verify created reconciliation
	reconEntities, err := s.transition.reconRepo.GetReconciliations(
		&reconciliation.WithRuntimeID{RuntimeID: clusterStates[0].Cluster.RuntimeID},
	)
	require.NoError(t, err)
	require.Len(t, reconEntities, 1)
	require.Greater(t, reconEntities[0].ClusterConfigStatus, oldClusterStateID) //verify new cluster-status ID is used
	require.False(t, reconEntities[0].Finished)

	//verify cluster status
	clusterState, err := s.transition.inventory.GetLatest(clusterStates[0].Cluster.RuntimeID)
	require.NoError(t, err)
	require.Equal(t, clusterState.Status.Status, model.ClusterStatusReconciling)
}

func (s *serviceTestSuite) TestTransitionFinishReconciliation() {
	t := s.T()
	clusterStates := s.prepareTransitionTest(t, 1)

	//get reconciliation entity
	reconEntities, err := s.transition.reconRepo.GetReconciliations(
		&reconciliation.WithRuntimeID{RuntimeID: clusterStates[0].Cluster.RuntimeID},
	)
	require.NoError(t, err)
	require.Len(t, reconEntities, 1)
	require.False(t, reconEntities[0].Finished)

	//finish the reconciliation
	err = s.transition.FinishReconciliation(reconEntities[0].SchedulingID, model.ClusterStatusReady)
	require.NoError(t, err)

	//finishing reconciliation twice is not allowed
	err = s.transition.FinishReconciliation(reconEntities[0].SchedulingID, model.ClusterStatusReady)
	require.Error(t, err)

	//verify that reconciliation is finished
	reconEntity, err := s.transition.reconRepo.GetReconciliation(reconEntities[0].SchedulingID)
	require.NoError(t, err)
	require.True(t, reconEntity.Finished)

	//verify cluster status
	clusterState, err := s.transition.inventory.GetLatest(clusterStates[0].Cluster.RuntimeID)
	require.NoError(t, err)
	require.Equal(t, clusterState.Status.Status, model.ClusterStatusReady)
}

func (s *serviceTestSuite) TestTransitionFinishWhenClusterNotInProgress() {
	t := s.T()
	clusterStates := s.prepareTransitionTest(t, 1)

	//get reconciliation entity
	reconEntities, err := s.transition.reconRepo.GetReconciliations(
		&reconciliation.WithRuntimeID{RuntimeID: clusterStates[0].Cluster.RuntimeID},
	)
	require.NoError(t, err)

	err = s.transition.FinishReconciliation(reconEntities[0].SchedulingID, model.ClusterStatusReady)
	require.NoError(t, err)

	//get reconciliation entity
	reconEntity, err := s.transition.ReconciliationRepository().CreateReconciliation(clusterStates[0], &model.ReconciliationSequenceConfig{
		PreComponents:  nil,
		DeleteStrategy: "",
	})
	require.NoError(t, err)
	require.NotNil(t, reconEntity)
	require.False(t, reconEntity.Finished)

	//retrieving cluster state
	currentClusterState, err := s.transition.inventory.GetLatest(clusterStates[0].Cluster.RuntimeID)
	require.NoError(t, err)

	//setting cluster state manually
	_, err = s.transition.inventory.UpdateStatus(currentClusterState, model.ClusterStatusDeletePending)
	require.NoError(t, err)

	//verify reconciliation success
	err = s.transition.FinishReconciliation(reconEntity.SchedulingID, model.ClusterStatusReady)
	require.NoError(t, err)

	//verify cluster status
	newClusterState, err := s.transition.inventory.GetLatest(clusterStates[0].Cluster.RuntimeID)
	require.NoError(t, err)
	require.Equal(t, model.ClusterStatusDeletePending, newClusterState.Status.Status)
}

func (s *serviceTestSuite) TestCleanDeletedClusters() {
	t := s.T()
	clusterStates := s.prepareTransitionTest(t, 5)

	for _, clusterState := range clusterStates {
		reconciliationEntities, err := s.transition.reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID})
		require.Len(t, reconciliationEntities, 1)
		require.NoError(t, err)
		require.NotNil(t, reconciliationEntities[0])
		require.False(t, reconciliationEntities[0].Finished)
		// set cluster and related configs, statuses as deleted
		err = s.transition.inventory.Delete(clusterState.Cluster.RuntimeID)
		require.NoError(t, err)
	}

	err := s.transition.CleanStatusesAndDeletedClustersOlderThan(time.Now(), 1, time.Millisecond*1)
	require.NoError(t, err)

	for _, clusterState := range clusterStates {
		state, err := s.transition.inventory.Get(clusterState.Configuration.RuntimeID, clusterState.Configuration.Version)
		require.Error(t, err)
		require.Nil(t, state)
	}
}
