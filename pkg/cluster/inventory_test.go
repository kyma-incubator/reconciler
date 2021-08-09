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
var clusterStatuses = []model.Status{model.Error, model.Ready, model.ReconcileFailed, model.ReconcilePending, model.Reconciling}

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

	// 	clusterState, err := inventory.Get(expectedCluster.Cluster, expectedVersion)
	// 	require.NoError(t, err)
	// 	compareState(t, clusterState, expectedCluster)
	// })

	t.Run("Get latest cluster", func(t *testing.T) {
		expectedCluster := newCluster(t, 1, maxVersion)

		clusterState, err := inventory.GetLatest(expectedCluster.Cluster)
		require.NoError(t, err)
		compareState(t, clusterState, expectedCluster)
	})

	t.Run("Update cluster status", func(t *testing.T) {
		cluster := newCluster(t, 1, maxVersion)
		clusterState, err := inventory.GetLatest(cluster.Cluster)
		require.NoError(t, err)
		require.Equal(t, clusterState.Status.Status, model.ReconcilePending)
		oldStatusID := clusterState.Status.ID
		//update status with same status (should NOT cause a status change)
		newState, err := inventory.UpdateStatus(clusterState, model.ReconcilePending)
		require.NoError(t, err)
		require.Equal(t, newState.Status.Status, model.ReconcilePending)
		require.Equal(t, oldStatusID, newState.Status.ID)
		//update status with new status (has to cause a status change)
		newState2, err := inventory.UpdateStatus(clusterState, model.Reconciling)
		require.NoError(t, err)
		require.Equal(t, newState2.Status.Status, model.Reconciling)
		require.True(t, oldStatusID < newState2.Status.ID)
	})

	t.Run("Delete a cluster", func(t *testing.T) {
		//get cluster1
		expectedCluster := newCluster(t, 1, 1)
		_, err := inventory.GetLatest(expectedCluster.Cluster)
		require.NoError(t, err)
		//delete cluster1
		require.NoError(t, inventory.Delete(expectedCluster.Cluster))
		//cluster1 is now missing
		_, err = inventory.GetLatest(expectedCluster.Cluster)
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
			_, err = inventory.UpdateStatus(clusterState, model.ReconcileFailed)
			require.NoError(t, err)
			//add expected status
			_, err = inventory.UpdateStatus(clusterState, clusterStatus)
			require.NoError(t, err)
		}

		defer func() {
			//cleanup
			for _, cluster := range expectedClusters {
				require.NoError(t, inventory.Delete(cluster.Cluster))
			}
		}()

		//check clusters to reconcile
		statesReconcile, err := inventory.ClustersToReconcile(0)
		require.NoError(t, err)
		require.Len(t, statesReconcile, 2)
		require.ElementsMatch(t,
			listStatuses(statesReconcile),
			[]model.Status{model.ReconcilePending, model.ReconcileFailed})

		//check clusters which are not ready
		statesNotReady, err := inventory.ClustersNotReady()
		require.NoError(t, err)
		require.Len(t, statesNotReady, 3)
		require.ElementsMatch(t,
			listStatuses(statesNotReady),
			[]model.Status{model.Reconciling, model.ReconcileFailed, model.Error})
	})

	t.Run("Edge-case: cluster has interim states and only latest state has to be replied)", func(t *testing.T) {
		inventory := newInventory(t)
		//create cluster1, version1, status: Ready
		cluster1v1 := newCluster(t, int64(1), 1)
		cluster1State1a, err := inventory.CreateOrUpdate(1, cluster1v1)
		require.NoError(t, err)
		require.Equal(t, model.ReconcilePending, cluster1State1a.Status.Status)
		cluster1State1b, err := inventory.UpdateStatus(cluster1State1a, model.Ready)
		require.NoError(t, err)
		require.Equal(t, model.Ready, cluster1State1b.Status.Status)

		//create cluster1, version2, status: ReconcileFailed
		cluster1v2 := newCluster(t, int64(1), 2)
		cluster1State2a, err := inventory.CreateOrUpdate(1, cluster1v2)
		require.NoError(t, err)
		require.Equal(t, model.ReconcilePending, cluster1State2a.Status.Status)
		cluster1State2b, err := inventory.UpdateStatus(cluster1State2a, model.ReconcileFailed)
		require.NoError(t, err)
		require.Equal(t, model.ReconcileFailed, cluster1State2b.Status.Status)

		//create cluster1, version3, status: ReconcilePending
		cluster1v3 := newCluster(t, int64(1), 3)
		expectedCluster1State3, err := inventory.CreateOrUpdate(1, cluster1v3) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ReconcilePending, expectedCluster1State3.Status.Status)

		//create cluster2, version1, status: Error
		cluster2v1 := newCluster(t, int64(2), 1)
		cluster2State1a, err := inventory.CreateOrUpdate(1, cluster2v1)
		require.NoError(t, err)
		require.Equal(t, model.ReconcilePending, cluster2State1a.Status.Status)
		cluster2State1b, err := inventory.UpdateStatus(cluster2State1a, model.Error)
		require.NoError(t, err)
		require.Equal(t, model.Error, cluster2State1b.Status.Status)

		//create cluster3, version1, status: Error
		cluster3v1 := newCluster(t, int64(3), 1)
		cluster3State1a, err := inventory.CreateOrUpdate(1, cluster3v1)
		require.NoError(t, err)
		require.Equal(t, model.ReconcilePending, cluster3State1a.Status.Status)
		cluster3State1b, err := inventory.UpdateStatus(cluster3State1a, model.Ready)
		require.NoError(t, err)
		require.Equal(t, model.Ready, cluster3State1b.Status.Status)
		expectedCluster3State1c, err := inventory.UpdateStatus(cluster3State1b, model.Error) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.Error, expectedCluster3State1c.Status.Status)

		//create cluster2, version2, status: ReconcileFailed
		cluster2v2 := newCluster(t, int64(2), 2)
		cluster2State2a, err := inventory.CreateOrUpdate(1, cluster2v2)
		require.NoError(t, err)
		require.Equal(t, model.ReconcilePending, cluster2State2a.Status.Status)
		expectedCluster2State2b, err := inventory.UpdateStatus(cluster2State2a, model.ReconcileFailed) //<- EXPECTED STATE
		require.NoError(t, err)
		require.Equal(t, model.ReconcileFailed, expectedCluster2State2b.Status.Status)

		defer func() {
			//cleanup
			for _, cluster := range []string{cluster1v3.Cluster, cluster2v2.Cluster, cluster3v1.Cluster} {
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
	})

	t.Run("Get status changes", func(t *testing.T) {
		inventory := newInventory(t)
		expectedStatuses := append(clusterStatuses, model.ReconcilePending)
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
			require.NoError(t, inventory.Delete(newCluster.Cluster))
		}()
		duration, err := time.ParseDuration("10h")
		require.NoError(t, err)
		changes, err := inventory.StatusChanges("cluster1", duration)
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
		result = append(result, *state.Status)
	}
	return result
}

type fakeMetricsCollector struct{}

func (collector fakeMetricsCollector) OnClusterStateUpdate(state *State) error {
	return nil
}

func newInventory(t *testing.T) Inventory {
	connFact, err := db.NewTestConnectionFactory()
	require.NoError(t, err)

	inventory, err := NewInventory(connFact, true, fakeMetricsCollector{})
	require.NoError(t, err)
	return inventory
}

func newCluster(t *testing.T, clusterID, clusterVersion int64) *keb.Cluster {
	cluster := &keb.Cluster{}
	data, err := ioutil.ReadFile(clusterJSONFile)
	require.NoError(t, err)
	err = json.Unmarshal(data, cluster)
	require.NoError(t, err)

	cluster.Cluster = fmt.Sprintf("cluster%d", clusterID)
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
	require.Equal(t, cluster.Cluster, state.Cluster.Cluster)
	//compare metadata
	require.Equal(t, toJSON(t, cluster.Metadata), state.Cluster.Metadata) //compare metadata-string
	metadata, err := state.Cluster.GetMetadata()
	require.NoError(t, err)
	require.Equal(t, &cluster.Metadata, metadata) //compare metadata-object
	//compare runtime
	require.Equal(t, toJSON(t, cluster.RuntimeInput), state.Cluster.Runtime) //compare runtime-string
	runtime, err := state.Cluster.GetRuntime()
	require.NoError(t, err)
	require.Equal(t, &cluster.RuntimeInput, runtime) //compare runtime-object

	// *** ClusterConfigurationEntity ***
	require.Equal(t, int64(1), state.Configuration.Contract)
	require.Equal(t, cluster.Cluster, state.Configuration.Cluster)
	require.Equal(t, cluster.KymaConfig.Profile, state.Configuration.KymaProfile)
	require.Equal(t, cluster.KymaConfig.Version, state.Configuration.KymaVersion)
	//compare components
	require.Equal(t, toJSON(t, cluster.KymaConfig.Components), state.Configuration.Components) //compare components-string
	components, err := state.Configuration.GetComponents()
	require.NoError(t, err)
	for _, comp := range components { //compare components-objects
		require.Contains(t, cluster.KymaConfig.Components, *comp)
	}
	//compare administrators
	require.Equal(t, toJSON(t, cluster.KymaConfig.Administrators), state.Configuration.Administrators) //compare admins-string
	admins, err := state.Configuration.GetAdministrators()
	require.NoError(t, err)
	require.Equal(t, cluster.KymaConfig.Administrators, admins) //compare components-object

	// *** ClusterStatusEntity ***
	require.Equal(t, model.ReconcilePending, state.Status.Status)
}

func toJSON(t *testing.T, model interface{}) string {
	result, err := json.Marshal(model)
	require.NoError(t, err)
	return string(result)
}
