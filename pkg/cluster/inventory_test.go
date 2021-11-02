package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/stretchr/testify/require"
)

const (
	maxVersion = 5
)

var clusterJSONFile = filepath.Join(".", "test", "cluster.json")
var clusterStatuses = []model.Status{
	model.ClusterStatusReconcileError, model.ClusterStatusReady, model.ClusterStatusReconcilePending, model.ClusterStatusReconciling,
	model.ClusterStatusDeleteError, model.ClusterStatusDeleted, model.ClusterStatusDeletePending, model.ClusterStatusDeleting}

func TestInventory(t *testing.T) {
	inventory := newInventory(t)

	t.Run("Create a cluster", func(t *testing.T) {
		//create cluster1
		expectedCluster := newCluster(t, 1, 1, false)
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

	t.Run("Update a cluster", func(t *testing.T) {
		//update cluster1 multiple times (will create multiple versions of it)
		for i := int64(2); i <= maxVersion; i++ { //"i" reflects cluster version
			expectedCluster := newCluster(t, 1, i, false)
			clusterState, err := inventory.CreateOrUpdate(1, expectedCluster)
			require.NoError(t, err)
			compareState(t, clusterState, expectedCluster)
		}
	})

	//FIXME: add support for cluster history to get previous versions
	// t.Run("Get specific cluster", func(t *testing.T) {
	// 	expectedVersion := int64(4) //NOT WORKING FOR POSTGRES
	// 	expectedCluster := newCluster(t, 1, expectedVersion)

	// 	clusterState, err := inventory.Get(expectedCluster.RuntimeID, expectedVersion)
	// 	require.NoError(t, err)
	// 	compareState(t, clusterState, expectedCluster)
	// })

	t.Run("Get latest cluster", func(t *testing.T) {
		expectedCluster := newCluster(t, 1, maxVersion, false)

		clusterState, err := inventory.GetLatest(expectedCluster.RuntimeID)
		require.NoError(t, err)
		compareState(t, clusterState, expectedCluster)
	})

	t.Run("Update cluster status", func(t *testing.T) {
		cluster := newCluster(t, 1, maxVersion, false)
		clusterState, err := inventory.GetLatest(cluster.RuntimeID)
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

	t.Run("Delete a cluster", func(t *testing.T) {
		//get cluster1
		expectedCluster := newCluster(t, 1, 1, false)
		_, err := inventory.GetLatest(expectedCluster.RuntimeID)
		require.NoError(t, err)
		//delete cluster1
		require.NoError(t, inventory.Delete(expectedCluster.RuntimeID))
		//cluster1 is now missing
		_, err = inventory.GetLatest(expectedCluster.RuntimeID)
		require.Error(t, err)
		require.True(t, repository.IsNotFoundError(err))
	})

	t.Run("Get clusters with particular status", func(t *testing.T) {
		var expectedClusters []*keb.Cluster

		// //create for each cluster-status a new cluster
		for idx, clusterStatus := range clusterStatuses {
			newCluster := newCluster(t, int64(idx+1), 1, false)
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

	t.Run("Get clusters to reconcile", func(t *testing.T) {
		inventory := newInventory(t)

		//create cluster1, clusterVersion1, clusterConfigVersion1-1, status: Ready
		cluster1v1v1 := newCluster(t, int64(1), 1, false)
		clusterState1v1v1a, err := inventory.CreateOrUpdate(1, cluster1v1v1)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, clusterState1v1v1a.Status.Status)
		clusterState1v1v1b, err := inventory.UpdateStatus(clusterState1v1v1a, model.ClusterStatusReady)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReady, clusterState1v1v1b.Status.Status)

		//create cluster1, clusterVersion2, clusterConfigVersion2-2, status: ReconcilePending
		cluster1v2v2 := newCluster(t, int64(1), 2, true)
		expectedClusterState1v2v2, err := inventory.CreateOrUpdate(1, cluster1v2v2) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, expectedClusterState1v2v2.Status.Status)

		//create cluster2, clusterVersion1, clusterConfigVersion1-1, status: ReconcilePending
		cluster2v1v1 := newCluster(t, int64(2), 1, false)
		clusterState2v1v1, err := inventory.CreateOrUpdate(1, cluster2v1v1)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, clusterState2v1v1.Status.Status)

		//create cluster2, clusterVersion1, clusterConfigVersion1-2, status: Error
		cluster2v1v2 := newCluster(t, int64(2), 1, true)
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
		cluster3v1v1 := newCluster(t, int64(3), 1, false)
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
		cluster4v1v1 := newCluster(t, int64(4), 1, false)
		_, err = inventory.CreateOrUpdate(1, cluster4v1v1)
		require.NoError(t, err)

		//create cluster4, clusterVersion1, clusterConfigVersion1-2, status: Ready
		cluster4v1v2 := newCluster(t, int64(4), 1, true)
		clusterState4v1v2, err := inventory.CreateOrUpdate(1, cluster4v1v2)
		require.NoError(t, err)
		_, err = inventory.UpdateStatus(clusterState4v1v2, model.ClusterStatusReady)
		require.NoError(t, err)

		//create cluster4, clusterVersion2, clusterConfigVersion1-1, status: ReconcilePending
		cluster4v2v1 := newCluster(t, int64(4), 2, false)
		clusterState4v2v1, err := inventory.CreateOrUpdate(1, cluster4v2v1)
		require.NoError(t, err)
		_, err = inventory.UpdateStatus(clusterState4v2v1, model.ClusterStatusReady)
		require.NoError(t, err)

		//create cluster4, clusterVersion2, clusterConfigVersion1-2, status: Ready
		cluster4v2v2 := newCluster(t, int64(4), 2, true)
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

	t.Run("Get status changes", func(t *testing.T) {
		inventory := newInventory(t)
		expectedStatuses := append(clusterStatuses, model.ClusterStatusReconcilePending)
		newCluster := newCluster(t, 1, 1, false)
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
		changes, err := inventory.StatusChanges("runtime1", duration)
		require.NoError(t, err)

		require.Len(t, changes, 9)
		require.ElementsMatch(t,
			listStatusesForStatusChanges(changes),
			expectedStatuses)
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

func newCluster(t *testing.T, runtimeID, clusterVersion int64, newConfigVersion bool) *keb.Cluster {
	cluster := &keb.Cluster{}
	data, err := ioutil.ReadFile(clusterJSONFile)
	require.NoError(t, err)
	err = json.Unmarshal(data, cluster)
	require.NoError(t, err)

	cluster.RuntimeID = fmt.Sprintf("runtime%d", runtimeID)
	cluster.RuntimeInput.Name = fmt.Sprintf("runtimeName%d", clusterVersion)
	cluster.Metadata.GlobalAccountID = fmt.Sprintf("globalAccountId%d", clusterVersion)
	cluster.Kubeconfig = "fake kubeconfig"

	var suffix string
	if newConfigVersion {
		suffix = fmt.Sprintf("%d_%s", clusterVersion, uuid.NewString()) //leads always to a new cluster-config entity
	} else {
		suffix = fmt.Sprintf("%d", clusterVersion)
	}
	cluster.KymaConfig.Profile = fmt.Sprintf("kymaProfile%s", suffix)
	cluster.KymaConfig.Version = fmt.Sprintf("kymaVersion%s", suffix)

	return cluster
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
