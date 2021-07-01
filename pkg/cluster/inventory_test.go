package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

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
		//create new entry
		expectedCluster := newCluster(t, 1, 1)
		clusterState, err := inventory.CreateOrUpdate(1, expectedCluster)
		require.NoError(t, err)
		compareState(t, clusterState, expectedCluster)
		//DB entities have to have ID 1
		require.Equal(t, int64(1), clusterState.Cluster.Version)
		require.Equal(t, int64(1), clusterState.Configuration.Version)
		require.Equal(t, int64(1), clusterState.Status.ID)

		//create same entry again (no new version should be created)
		clusterStateNew, err := inventory.CreateOrUpdate(1, expectedCluster)
		require.NoError(t, err)
		compareState(t, clusterStateNew, expectedCluster)
		//DB entities have to have again ID 1
		require.Equal(t, int64(1), clusterStateNew.Cluster.Version)
		require.Equal(t, int64(1), clusterStateNew.Configuration.Version)
		require.Equal(t, int64(1), clusterStateNew.Status.ID)
	})

	t.Run("Update a cluster", func(t *testing.T) {
		//update a cluster multiple times (will create multiple versions of it)
		for i := int64(2); i <= maxVersion; i++ { //"i" reflects also the expected DB ID
			expectedCluster := newCluster(t, 1, i)
			clusterState, err := inventory.CreateOrUpdate(1, expectedCluster)
			require.NoError(t, err)
			compareState(t, clusterState, expectedCluster)
			//DB entities have to have an incremented DB ID
			require.Equal(t, i, clusterState.Cluster.Version)
			require.Equal(t, i, clusterState.Configuration.Version)
			require.Equal(t, i, clusterState.Status.ID)
		}
	})

	t.Run("Get specific cluster", func(t *testing.T) {
		expectedVersion := int64(4)
		expectedCluster := newCluster(t, 1, expectedVersion)

		clusterState, err := inventory.Get(expectedCluster.Cluster, expectedVersion)
		require.NoError(t, err)
		compareState(t, clusterState, expectedCluster)
	})

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
		//get existing cluster
		expectedCluster := newCluster(t, 1, 1)
		_, err := inventory.GetLatest(expectedCluster.Cluster)
		require.NoError(t, err)
		//delete cluster
		require.NoError(t, inventory.Delete(expectedCluster.Cluster))
		//cluster is now missing
		_, err = inventory.GetLatest(expectedCluster.Cluster)
		require.Error(t, err)
		require.True(t, repository.IsNotFoundError(err))
	})

	t.Run("Get clusters with particular status", func(t *testing.T) {
		// //create for each cluster-status a new cluster
		for idx, clusterStatus := range clusterStatuses {
			expectedCluster := newCluster(t, int64(idx+100), 1)
			clusterState, err := inventory.CreateOrUpdate(1, expectedCluster)
			require.NoError(t, err)
			_, err = inventory.UpdateStatus(clusterState, clusterStatus)
			require.NoError(t, err)
		}
		statesReconcile, err := inventory.ClustersToReconcile() //TODO: check particular states and cluster IDs
		require.NoError(t, err)
		require.Len(t, statesReconcile, 2) //TODO: check particular states and cluster IDs
		statesNotReady, err := inventory.ClustersNotReady()
		require.NoError(t, err)
		require.Len(t, statesNotReady, 3) //TODO: check particular states and cluster IDs
	})
}

func newInventory(t *testing.T) Inventory {
	connFact, err := db.NewTestConnectionFactory()
	require.NoError(t, err)
	inventory, err := NewInventory(connFact, true)
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
