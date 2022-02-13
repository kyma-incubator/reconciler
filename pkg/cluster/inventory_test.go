package cluster

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
	"time"
)

const (
	maxVersion = 5
)

var clusterStatuses = []model.Status{
	model.ClusterStatusReconcileError, model.ClusterStatusReady, model.ClusterStatusReconcilePending, model.ClusterStatusReconciling,
	model.ClusterStatusDeleteError, model.ClusterStatusDeleted, model.ClusterStatusDeletePending, model.ClusterStatusDeleting}

func TestInventory(t *testing.T) {
	inventory := newInventory(t)

	expectedCluster := test.NewCluster(t, "1", 1, false, test.Production)

	removeAllClusters(t, inventory)       //cleanup before the test runs
	defer removeAllClusters(t, inventory) //cleanup after test is finished

	t.Run("Create expectedCluster", func(t *testing.T) {
		//create cluster1
		clusterState, err := inventory.CreateOrUpdate(1, expectedCluster)
		require.NoError(t, err)
		compareState(t, clusterState, expectedCluster)

		//create same entry again (no new version should be created)
		clusterStateNew, err := inventory.CreateOrUpdate(1, expectedCluster)
		require.NoError(t, err)
		require.Equal(t, clusterState.Cluster.Version, clusterStateNew.Cluster.Version)
		require.Equal(t, clusterState.Configuration.Version, clusterStateNew.Configuration.Version)
		require.Equal(t, clusterState.Status.ID, clusterStateNew.Status.ID)
		compareState(t, clusterStateNew, expectedCluster)
	})

	t.Run("Update expectedCluster to maxVersion", func(t *testing.T) {
		//update cluster1 multiple times (will create multiple versions of it)
		for i := uint64(2); i <= maxVersion; i++ { //"i" reflects cluster version
			updatedCluster := test.NewClusterFromExisting(*expectedCluster, i, false)
			clusterState, err := inventory.CreateOrUpdate(1, updatedCluster)
			require.NoError(t, err)
			compareState(t, clusterState, updatedCluster)
		}
	})

	//FIXME: add support for cluster history to get previous versions
	// t.Run("Get specific cluster", func(t *testing.T) {
	// 	expectedVersion := int64(4) //NOT WORKING FOR POSTGRES
	// 	expectedCluster := keb.NewCluster(t, 1, expectedVersion)

	// 	clusterState, err := inventory.Get(expectedCluster.RuntimeID, expectedVersion)
	// 	require.NoError(t, err)
	// 	compareState(t, clusterState, expectedCluster)
	// })

	t.Run("Get latest cluster with maxVersion", func(t *testing.T) {
		latestCluster := test.NewClusterFromExisting(*expectedCluster, maxVersion, false)
		clusterState, err := inventory.GetLatest(latestCluster.RuntimeID)
		require.NoError(t, err)
		compareState(t, clusterState, latestCluster)
	})

	t.Run("Update cluster status", func(t *testing.T) {
		latestCluster := test.NewClusterFromExisting(*expectedCluster, maxVersion, false)
		clusterState, err := inventory.GetLatest(latestCluster.RuntimeID)
		require.NoError(t, err)
		require.Equal(t, clusterState.Status.Status, model.ClusterStatusReconcilePending)
		oldStatusID := clusterState.Status.ID
		//update status with same status (should NOT cause a status change)
		newState, err := inventory.UpdateStatus(clusterState, model.ClusterStatusReconcilePending)
		require.NoError(t, err)
		require.Equal(t, newState.Status.Status, model.ClusterStatusReconcilePending)
		require.Equal(t, oldStatusID, newState.Status.ID)
		//update status with new status (has to cause a status change)
		newState2, err := inventory.UpdateStatus(clusterState, model.ClusterStatusReconciling)
		require.NoError(t, err)
		require.Equal(t, newState2.Status.Status, model.ClusterStatusReconciling)
		require.True(t, oldStatusID < newState2.Status.ID)
	})

	t.Run("Get all", func(t *testing.T) {
		//verify the expected cluster is returned
		clustersOld, err := inventory.GetAll()
		require.NoError(t, err)
		require.Len(t, clustersOld, 1)

		//add a new cluster
		newCluster, err := inventory.CreateOrUpdate(1, test.NewCluster(t, "2", 1, false, test.Production))
		require.NoError(t, err)

		//check that both clusters are now returned
		clustersNew, err := inventory.GetAll()
		require.NoError(t, err)
		require.Len(t, clustersNew, 2)
		require.Contains(t, clustersNew, newCluster)

		//verify that just the expected cluster is returned after removing the new created cluster
		require.NoError(t, inventory.Delete(newCluster.Cluster.RuntimeID))
		clusterOldRefershed, err := inventory.GetAll()
		require.NoError(t, err)
		require.ElementsMatch(t, clustersOld, clusterOldRefershed)
	})

	t.Run("Delete expectedCluster", func(t *testing.T) {
		_, err := inventory.GetLatest(expectedCluster.RuntimeID)
		require.NoError(t, err)
		//delete cluster1
		require.NoError(t, inventory.Delete(expectedCluster.RuntimeID))
		//cluster1 is now missing
		_, err = inventory.GetLatest(expectedCluster.RuntimeID)
		require.Error(t, err)
		require.True(t, repository.IsNotFoundError(err))
	})

}
func TestInventoryForClusterStatues(t *testing.T) {
	inventory := newInventory(t)

	t.Run("Get clusters with particular status", func(t *testing.T) {
		var expectedClusters []*keb.Cluster

		// //create for each cluster-status a new cluster
		for idx, clusterStatus := range clusterStatuses {
			newCluster := test.NewCluster(t, strconv.Itoa(idx+1), 1, false, test.Production)
			clusterState, err := inventory.CreateOrUpdate(1, newCluster)
			require.NoError(t, err)
			expectedClusters = append(expectedClusters, newCluster)
			//add another status to verify that SQL query works correctly
			_, err = inventory.UpdateStatus(clusterState, model.ClusterStatusReconcileError)
			require.NoError(t, err)
			//add expected status
			_, err = inventory.UpdateStatus(clusterState, clusterStatus)
			require.NoError(t, err)
		}

		defer func() {
			//cleanup
			for _, cluster := range expectedClusters {
				require.NoError(t, inventory.Delete(cluster.RuntimeID))
			}
		}()

		//check clusters to reconcile
		statesReconcile, err := inventory.ClustersToReconcile(0)
		require.NoError(t, err)
		require.Len(t, statesReconcile, 2)
		require.ElementsMatch(t,
			listStatuses(statesReconcile),
			[]model.Status{model.ClusterStatusReconcilePending, model.ClusterStatusDeletePending})

		//check clusters which are not ready
		statesNotReady, err := inventory.ClustersNotReady()
		require.NoError(t, err)
		require.Len(t, statesNotReady, 4)
		require.ElementsMatch(t,
			listStatuses(statesNotReady),
			[]model.Status{model.ClusterStatusReconciling, model.ClusterStatusReconcileError, model.ClusterStatusDeleting, model.ClusterStatusDeleteError})
	})
	t.Run("Get status changes", func(t *testing.T) {
		expectedStatuses := append(clusterStatuses, model.ClusterStatusReconcilePending)
		newCluster := test.NewCluster(t, "1", 1, false, test.Production)
		clusterState, err := inventory.CreateOrUpdate(1, newCluster)
		require.NoError(t, err)
		// //create for each cluster-status a new cluster
		for _, clusterStatus := range clusterStatuses {
			//add expected status
			_, err = inventory.UpdateStatus(clusterState, clusterStatus)
			require.NoError(t, err)
		}

		defer func() {
			//cleanup
			require.NoError(t, inventory.Delete(newCluster.RuntimeID))
		}()
		duration, err := time.ParseDuration("10h")
		require.NoError(t, err)
		changes, err := inventory.StatusChanges(newCluster.RuntimeID, duration)
		require.NoError(t, err)

		require.Len(t, changes, 9)
		require.ElementsMatch(t,
			listStatusesForStatusChanges(changes),
			expectedStatuses)
	})

}
func TestInventoryForReconcile(t *testing.T) {
	inventory := newInventory(t)
	t.Run("Get clusters to reconcile", func(t *testing.T) {
		//create cluster1, clusterVersion1, clusterConfigVersion1-1, status: Ready
		cluster1v1v1 := test.NewCluster(t, "1", 1, false, test.Production)
		clusterState1v1v1a, err := inventory.CreateOrUpdate(1, cluster1v1v1)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, clusterState1v1v1a.Status.Status)
		clusterState1v1v1b, err := inventory.UpdateStatus(clusterState1v1v1a, model.ClusterStatusReady)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReady, clusterState1v1v1b.Status.Status)

		//create cluster1, clusterVersion2, clusterConfigVersion2-2, status: ReconcilePending
		cluster1v2v2 := test.NewClusterFromExisting(*cluster1v1v1, 2, true)
		expectedClusterState1v2v2, err := inventory.CreateOrUpdate(1, cluster1v2v2) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, expectedClusterState1v2v2.Status.Status)

		//create cluster2, clusterVersion1, clusterConfigVersion1-1, status: ReconcilePending
		cluster2v1v1 := test.NewCluster(t, "2", 1, false, test.Production)
		clusterState2v1v1, err := inventory.CreateOrUpdate(1, cluster2v1v1)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, clusterState2v1v1.Status.Status)

		//create cluster2, clusterVersion1, clusterConfigVersion1-2, status: Error
		cluster2v1v2 := test.NewClusterFromExisting(*cluster2v1v1, 1, true)
		clusterState2v1v2a, err := inventory.CreateOrUpdate(1, cluster2v1v2)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, clusterState2v1v2a.Status.Status)
		clusterState2v1v2b, err := inventory.UpdateStatus(clusterState2v1v2a, model.ClusterStatusReconcileError)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcileError, clusterState2v1v2b.Status.Status)

		//delete cluster2, status: DeletePending -> Deleting
		cluster2State2a, err := inventory.MarkForDeletion(cluster2v1v2.RuntimeID)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusDeletePending, cluster2State2a.Status.Status)
		expectedCluster2State2b, err := inventory.UpdateStatus(cluster2State2a, model.ClusterStatusDeleting) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusDeleting, expectedCluster2State2b.Status.Status)

		//create cluster3, clusterVersion1, clusterConfigVersion1-1, status: Error
		cluster3v1v1 := test.NewCluster(t, "3", 1, false, test.Production)
		clusterState3v1v1a, err := inventory.CreateOrUpdate(1, cluster3v1v1)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, clusterState3v1v1a.Status.Status)
		clusterState3v1v1b, err := inventory.UpdateStatus(clusterState3v1v1a, model.ClusterStatusReady)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReady, clusterState3v1v1b.Status.Status)
		expectedClusterState3v1v1c, err := inventory.UpdateStatus(clusterState3v1v1b, model.ClusterStatusReconcileError) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcileError, expectedClusterState3v1v1c.Status.Status)

		//create cluster4, clusterVersion1, clusterConfigVersion1-1, status: ReconcilePending
		cluster4v1v1 := test.NewCluster(t, "4", 1, false, test.Production)
		_, err = inventory.CreateOrUpdate(1, cluster4v1v1)
		require.NoError(t, err)

		//create cluster4, clusterVersion1, clusterConfigVersion1-2, status: Ready
		cluster4v1v2 := test.NewClusterFromExisting(*cluster4v1v1, 1, true)
		clusterState4v1v2, err := inventory.CreateOrUpdate(1, cluster4v1v2)
		require.NoError(t, err)
		_, err = inventory.UpdateStatus(clusterState4v1v2, model.ClusterStatusReady)
		require.NoError(t, err)

		//create cluster4, clusterVersion2, clusterConfigVersion1-1, status: ReconcilePending
		cluster4v2v1 := test.NewClusterFromExisting(*cluster4v1v1, 2, false)
		clusterState4v2v1, err := inventory.CreateOrUpdate(1, cluster4v2v1)
		require.NoError(t, err)
		_, err = inventory.UpdateStatus(clusterState4v2v1, model.ClusterStatusReady)
		require.NoError(t, err)

		//create cluster4, clusterVersion2, clusterConfigVersion1-2, status: Ready
		cluster4v2v2 := test.NewClusterFromExisting(*cluster4v2v1, 2, true)
		clusterState4v2v2a, err := inventory.CreateOrUpdate(1, cluster4v2v2)
		require.NoError(t, err)
		expectedClusterState4v2v2b, err := inventory.UpdateStatus(clusterState4v2v2a, model.ClusterStatusReady) //<-EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReady, expectedClusterState4v2v2b.Status.Status)

		defer func() {
			//cleanup
			for _, cluster := range []string{cluster1v2v2.RuntimeID, cluster2v1v2.RuntimeID, cluster3v1v1.RuntimeID, cluster4v2v2.RuntimeID} {
				require.NoError(t, inventory.Delete(cluster))
			}
		}()

		time.Sleep(2 * time.Second) //wait 2 sec to ensure cluster 4 exceeds the reconciliation timeout

		//get clusters to reconcile
		statesReconcile, err := inventory.ClustersToReconcile(1 * time.Second)
		require.NoError(t, err)
		require.Len(t, statesReconcile, 2)
		require.ElementsMatch(t, []*State{expectedClusterState1v2v2, expectedClusterState4v2v2b}, statesReconcile)

		//get clusters in not ready state
		statesNotReady, err := inventory.ClustersNotReady()
		require.NoError(t, err)
		require.Len(t, statesNotReady, 2)
		require.ElementsMatch(t, []*State{expectedCluster2State2b, expectedClusterState3v1v1c}, statesNotReady)

	})
}

func TestCountRetries(t *testing.T) {
	expectedCluster1 := test.NewCluster(t, "1", 1, false, test.Production)
	expectedCluster2 := test.NewCluster(t, "2", 2, false, test.Production)
	expectedCluster3 := test.NewCluster(t, "3", 3, false, test.Production)

	inventory := newInventory(t)
	defer func() {
		for _, runtimeID := range []string{
			expectedCluster1.RuntimeID,
			expectedCluster2.RuntimeID,
			expectedCluster3.RuntimeID,
		} {
			require.NoError(t, inventory.Delete(runtimeID))
		}
	}()

	t.Run("When there are retry before status ready, expect to be skipped and not take into count.", func(t *testing.T) {
		clusterState, err := inventory.CreateOrUpdate(1, expectedCluster1)
		require.NoError(t, err)

		const errorStatus = model.ClusterStatusReconcileErrorRetryable
		skippedRetry := 10
		for i := 0; i < skippedRetry; i++ {
			clusterState, err = inventory.UpdateStatus(clusterState, errorStatus)
			require.NoError(t, err)
		}

		clusterState, err = inventory.UpdateStatus(clusterState, model.ClusterStatusReady)
		require.NoError(t, err)
		cnt, err := inventory.CountRetries(clusterState.Configuration.RuntimeID, clusterState.Configuration.Version, 10, errorStatus)
		require.NoError(t, err)

		require.Equal(t, 0, cnt)
	})

	t.Run("When there are retry after status ready, expect to be counted.", func(t *testing.T) {
		expectedErrRetryable := 50

		clusterState, err := inventory.CreateOrUpdate(2, expectedCluster2)
		require.NoError(t, err)

		const errorStatusTobeCounted = model.ClusterStatusReconcileErrorRetryable

		clusterState, err = inventory.UpdateStatus(clusterState, model.ClusterStatusReady)
		require.NoError(t, err)

		for i := 0; i < expectedErrRetryable; i++ {
			clusterState, err = inventory.UpdateStatus(clusterState, errorStatusTobeCounted)
			require.NoError(t, err)
			clusterState, err = inventory.UpdateStatus(clusterState, model.ClusterStatusReconciling)
			require.NoError(t, err)
		}

		cnt, err := inventory.CountRetries(clusterState.Configuration.RuntimeID, clusterState.Configuration.Version, 150, errorStatusTobeCounted)
		require.NoError(t, err)
		require.Equal(t, expectedErrRetryable, cnt)
	})

	t.Run("When there are not expected error status, expect them to be skipped and only count expected status", func(t *testing.T) {
		expectedErrRetryable := 50

		clusterState, err := inventory.CreateOrUpdate(3, expectedCluster3)
		require.NoError(t, err)

		const errorStatusTobeCounted = model.ClusterStatusReconcileErrorRetryable
		const errorStatusNotTobCounted = model.ClusterStatusReconciling

		for i := 0; i < expectedErrRetryable; i++ {
			clusterState, err = inventory.UpdateStatus(clusterState, errorStatusTobeCounted)
			require.NoError(t, err)
			clusterState, err = inventory.UpdateStatus(clusterState, errorStatusNotTobCounted)
			require.NoError(t, err)
		}
		cnt, err := inventory.CountRetries(clusterState.Configuration.RuntimeID, clusterState.Configuration.Version, 150, errorStatusTobeCounted)
		require.NoError(t, err)
		require.Equal(t, expectedErrRetryable, cnt)
	})
}

func TestTransaction(t *testing.T) {
	t.Run("Rollback nested transactions", func(t *testing.T) {

		//new db connection
		dbConn := db.NewTestConnection(t)

		//create inventory
		inventory, err := NewInventory(dbConn, true, MetricsCollectorMock{})

		require.NoError(t, err)
		var clusterState *State
		var clusterState2 *State
		defer func() {
			//cleanup
			require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
			require.NoError(t, inventory.Delete(clusterState2.Cluster.RuntimeID))
		}()
		dbOp := func(tx *db.TxConnection) error {

			//converts inventory with given tx
			inventory, err := inventory.WithTx(tx)
			require.NoError(t, err)

			//create two clusters
			clusterState, err = inventory.CreateOrUpdate(1, test.NewCluster(t, "neverExist1", 1, false, test.OneComponentDummy))
			require.NoError(t, err)
			clusterState2, err = inventory.CreateOrUpdate(1, test.NewCluster(t, "neverExist2", 1, false, test.OneComponentDummy))
			require.NoError(t, err)

			//check if clusters are created
			state, err := inventory.Get(clusterState.Cluster.RuntimeID, clusterState.Configuration.Version)
			require.NoError(t, err)
			require.NotNil(t, state)
			state2, err := inventory.Get(clusterState2.Cluster.RuntimeID, clusterState2.Configuration.Version)
			require.NoError(t, err)
			require.NotNil(t, state2)

			//rollback transactions
			return errors.New("Faking an error")
		}
		require.Error(t, db.Transaction(dbConn, dbOp, logger.NewLogger(true)))

		//check if cluster creations are rolled back
		state, err := inventory.Get(clusterState.Cluster.RuntimeID, clusterState.Configuration.Version)
		require.Error(t, err)
		require.True(t, repository.IsNotFoundError(err))
		require.Nil(t, state)
		state2, err := inventory.Get(clusterState2.Cluster.RuntimeID, clusterState2.Configuration.Version)
		require.Error(t, err)
		require.True(t, repository.IsNotFoundError(err))
		require.Nil(t, state2)
	})
}

func listStatuses(states []*State) []model.Status {
	var result []model.Status
	for _, state := range states {
		result = append(result, state.Status.Status)
	}
	return result
}

func listStatusesForStatusChanges(states []*StatusChange) []model.Status {
	var result []model.Status
	for _, state := range states {
		result = append(result, state.Status.Status)
	}
	return result
}

func newInventory(t *testing.T) Inventory {
	inventory, err := NewInventory(db.NewTestConnection(t), true, MetricsCollectorMock{})
	require.NoError(t, err)
	return inventory
}

func compareState(t *testing.T, state *State, cluster *keb.Cluster) {
	// *** ClusterEntity ***
	require.Equal(t, int64(1), state.Cluster.Contract)
	require.Equal(t, cluster.RuntimeID, state.Cluster.RuntimeID)
	//compare metadata
	require.Equal(t, &cluster.Metadata, state.Cluster.Metadata) //compare metadata-string

	//compare runtime
	require.Equal(t, &cluster.RuntimeInput, state.Cluster.Runtime) //compare runtime-string

	// *** ClusterConfigurationEntity ***
	require.Equal(t, int64(1), state.Configuration.Contract)
	require.Equal(t, cluster.RuntimeID, state.Configuration.RuntimeID)
	require.Equal(t, cluster.KymaConfig.Profile, state.Configuration.KymaProfile)
	require.Equal(t, cluster.KymaConfig.Version, state.Configuration.KymaVersion)
	//compare components
	require.ElementsMatch(t, func() []*keb.Component {
		var result []*keb.Component
		for idx := range cluster.KymaConfig.Components {
			result = append(result, &cluster.KymaConfig.Components[idx])
		}
		return result
	}(), state.Configuration.Components)
	require.Len(t, cluster.KymaConfig.Components, 7)

	//compare administrators
	require.Equal(t, cluster.KymaConfig.Administrators, state.Configuration.Administrators) //compare admins-string

	// *** ClusterStatusEntity ***
	require.Equal(t, model.ClusterStatusReconcilePending, state.Status.Status)
}

func removeAllClusters(t *testing.T, inventory Inventory) {
	t.Log("Remove all clusters from inventory")
	allClusters, err := inventory.GetAll()
	require.NoError(t, err)
	for _, cluster := range allClusters {
		require.NoError(t, inventory.Delete(cluster.Cluster.RuntimeID))
	}
}
