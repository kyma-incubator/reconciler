package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

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
var clusterStatuses = []model.Status{model.ClusterStatusError, model.ClusterStatusReady, model.ClusterStatusReconcileFailed, model.ClusterStatusReconcilePending, model.ClusterStatusReconciling}

func TestInventory(t *testing.T) {
	inventory := newInventory(t)

	t.Run("Create a cluster", func(t *testing.T) {
		//create cluster1
		expectedCluster := newCluster(t, 1, 1)
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
			expectedCluster := newCluster(t, 1, i)
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
		expectedCluster := newCluster(t, 1, maxVersion)

		clusterState, err := inventory.GetLatest(expectedCluster.RuntimeID)
		require.NoError(t, err)
		compareState(t, clusterState, expectedCluster)
	})

	t.Run("Update cluster status", func(t *testing.T) {
		cluster := newCluster(t, 1, maxVersion)
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
		expectedCluster := newCluster(t, 1, 1)
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
			newCluster := newCluster(t, int64(idx+1), 1)
			clusterState, err := inventory.CreateOrUpdate(1, newCluster)
			require.NoError(t, err)
			expectedClusters = append(expectedClusters, newCluster)
			//add another status to verify that SQL query works correctly
			_, err = inventory.UpdateStatus(clusterState, model.ClusterStatusReconcileFailed)
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
			[]model.Status{model.ClusterStatusReconcilePending, model.ClusterStatusReconcileFailed})

		//check clusters which are not ready
		statesNotReady, err := inventory.ClustersNotReady()
		require.NoError(t, err)
		require.Len(t, statesNotReady, 3)
		require.ElementsMatch(t,
			listStatuses(statesNotReady),
			[]model.Status{model.ClusterStatusReconciling, model.ClusterStatusReconcileFailed, model.ClusterStatusError})
	})

	t.Run("Edge-case: cluster has interim states and only latest state has to be replied)", func(t *testing.T) {
		inventory := newInventory(t)
		//create cluster1, version1, status: Ready
		cluster1v1 := newCluster(t, int64(1), 1)
		cluster1State1a, err := inventory.CreateOrUpdate(1, cluster1v1)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, cluster1State1a.Status.Status)
		cluster1State1b, err := inventory.UpdateStatus(cluster1State1a, model.ClusterStatusReady)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReady, cluster1State1b.Status.Status)

		//create cluster1, version2, status: ReconcileFailed
		cluster1v2 := newCluster(t, int64(1), 2)
		cluster1State2a, err := inventory.CreateOrUpdate(1, cluster1v2)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, cluster1State2a.Status.Status)
		cluster1State2b, err := inventory.UpdateStatus(cluster1State2a, model.ClusterStatusReconcileFailed)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcileFailed, cluster1State2b.Status.Status)

		//create cluster1, version3, status: ReconcilePending
		cluster1v3 := newCluster(t, int64(1), 3)
		expectedCluster1State3, err := inventory.CreateOrUpdate(1, cluster1v3) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, expectedCluster1State3.Status.Status)

		//create cluster2, version1, status: Error
		cluster2v1 := newCluster(t, int64(2), 1)
		cluster2State1a, err := inventory.CreateOrUpdate(1, cluster2v1)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, cluster2State1a.Status.Status)
		cluster2State1b, err := inventory.UpdateStatus(cluster2State1a, model.ClusterStatusError)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusError, cluster2State1b.Status.Status)

		//create cluster3, version1, status: Error
		cluster3v1 := newCluster(t, int64(3), 1)
		cluster3State1a, err := inventory.CreateOrUpdate(1, cluster3v1)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, cluster3State1a.Status.Status)
		cluster3State1b, err := inventory.UpdateStatus(cluster3State1a, model.ClusterStatusReady)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReady, cluster3State1b.Status.Status)
		expectedCluster3State1c, err := inventory.UpdateStatus(cluster3State1b, model.ClusterStatusError) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusError, expectedCluster3State1c.Status.Status)

		//create cluster2, version2, status: ReconcileFailed
		cluster2v2 := newCluster(t, int64(2), 2)
		cluster2State2a, err := inventory.CreateOrUpdate(1, cluster2v2)
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcilePending, cluster2State2a.Status.Status)
		expectedCluster2State2b, err := inventory.UpdateStatus(cluster2State2a, model.ClusterStatusReconcileFailed) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ClusterStatusReconcileFailed, expectedCluster2State2b.Status.Status)

		defer func() {
			//cleanup
			for _, cluster := range []string{cluster1v3.RuntimeID, cluster2v2.RuntimeID, cluster3v1.RuntimeID} {
				require.NoError(t, inventory.Delete(cluster))
			}
		}()

		//get clusters to reconcile
		statesReconcile, err := inventory.ClustersToReconcile(0)
		require.NoError(t, err)
		require.Len(t, statesReconcile, 2)
		require.ElementsMatch(t, []*State{expectedCluster1State3, expectedCluster2State2b}, statesReconcile)

		//get clusters in not ready state
		statesNotReady, err := inventory.ClustersNotReady()
		require.NoError(t, err)
		require.Len(t, statesNotReady, 2)
		require.ElementsMatch(t, []*State{expectedCluster2State2b, expectedCluster3State1c}, statesNotReady)

		//TODO: test for clusters which are inside and outside of filter interval
	})

	t.Run("Get status changes", func(t *testing.T) {
		inventory := newInventory(t)
		expectedStatuses := append(clusterStatuses, model.ClusterStatusReconcilePending)
		newCluster := newCluster(t, 1, 1)
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

		require.Len(t, changes, 6)
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

func newCluster(t *testing.T, runtimeID, clusterVersion int64) *keb.Cluster {
	cluster := &keb.Cluster{}
	data, err := ioutil.ReadFile(clusterJSONFile)
	require.NoError(t, err)
	err = json.Unmarshal(data, cluster)
	require.NoError(t, err)

	cluster.RuntimeID = fmt.Sprintf("runtime%d", runtimeID)
	cluster.RuntimeInput.Name = fmt.Sprintf("runtimeName%d", clusterVersion)
	cluster.Metadata.GlobalAccountID = fmt.Sprintf("globalAccountId%d", clusterVersion)
	cluster.KymaConfig.Profile = fmt.Sprintf("kymaProfile%d", clusterVersion)
	cluster.KymaConfig.Version = fmt.Sprintf("kymaVersion%d", clusterVersion)
	cluster.Kubeconfig = "fake kubeconfig"

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
