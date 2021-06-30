package cluster

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestInventory(t *testing.T) {
	t.Run("Create a cluster", func(t *testing.T) {
		clusterModel := newCluster(t)
		state, err := newInventory(t).CreateOrUpdate(1, clusterModel)
		require.NoError(t, err)
		compareState(t, state, clusterModel)
	})
	t.Run("Update a cluster", func(t *testing.T) {})
	t.Run("Get specific cluster", func(t *testing.T) {})
	t.Run("Get latest cluster", func(t *testing.T) {})
	t.Run("Update cluster status", func(t *testing.T) {})
	t.Run("Delete a cluster", func(t *testing.T) {})
	t.Run("Get clusters to reconcile", func(t *testing.T) {})
}

func newInventory(t *testing.T) Inventory {
	connFact, err := db.NewTestConnectionFactory()
	require.NoError(t, err)
	inventory, err := NewInventory(connFact, true)
	require.NoError(t, err)
	return inventory
}

func newCluster(t *testing.T) *keb.Cluster {
	cluster := &keb.Cluster{}
	data, err := ioutil.ReadFile(filepath.Join(".", "test", "cluster.json"))
	require.NoError(t, err)
	err = json.Unmarshal(data, cluster)
	require.NoError(t, err)
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
