package chart

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
	"github.com/stretchr/testify/require"
)

var (
	componentListFile         string = filepath.Join("test", "unittest-complist.yaml")
	componentListExpectedFile string = filepath.Join("test", "unittest-complist-expected.yaml")
)

func TestProvider(t *testing.T) {
	wsFactory := &workspace.Factory{
		StorageDir: "./test",
	}
	prov, err := NewProvider(wsFactory, true)
	require.NoError(t, err)

	t.Run("Convert KEB configuration to a map", func(t *testing.T) {
		got := prov.kebConfToMap(keb.Configuration{
			Key:   "this.is.a.test",
			Value: "the test value",
		})
		expected := make(map[string]interface{})
		err := json.Unmarshal([]byte(`{
			"this":{
				"is":{
					"a":{
						"test":"the test value"
					}
				}
			}
		}`), &expected) //use marshaller for convenience
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("Test overrides handling", func(t *testing.T) {
		builder, err := prov.overrides([]*keb.Components{
			{
				Component: "test-component",
				Configuration: []keb.Configuration{
					{
						Key:   "test.key1.subkey1",
						Value: "test value 1",
					},
					{
						Key:   "test.key1.subkey2",
						Value: "test value 2",
					},
					{
						Key:   "test.key2.subkey1",
						Value: "test value 3",
					},
				},
			},
		})
		require.NoError(t, err)
		overrides, err := builder.Build()
		require.NoError(t, err)
		overridesMap := overrides.Map()

		expected := make(map[string]interface{})
		err = json.Unmarshal([]byte(`{
			"test-component":{
				"test":{
					"key1":{
						"subkey1":"test value 1",
						"subkey2":"test value 2"
					},
					"key2":{
						"subkey1":"test value 3"
					}
				}
			}
		}`), &expected) //use marshaller for convenience
		require.NoError(t, err)
		require.Equal(t, expected, overridesMap)
	})

	t.Run("Test component list", func(t *testing.T) {
		compList, err := prov.componentList(&workspace.Workspace{
			ComponentFile: componentListFile,
		}, []*keb.Components{
			{
				Component: "component-2",
				Namespace: "differentns-component-2",
			},
			{
				Component: "component-3",
				Namespace: "differentns-component-3",
			},
		})
		require.NoError(t, err)

		expCompList, err := config.NewComponentList(componentListExpectedFile)
		require.NoError(t, err)

		require.Equal(t, expCompList, compList)
	})

	t.Run("Test component list", func(t *testing.T) {
		if !test.RunExpensiveTests() {
			return
		}
		kubeCfg, err := ioutil.ReadFile(filepath.Join("test", "unittest-kubeconfig.yaml"))
		require.NoError(t, err)

		manifests, err := prov.renderManifests(
			&cluster.State{
				Cluster: &model.ClusterEntity{
					Version:  1,
					Cluster:  "cluster1",
					Contract: 1,
				},
				Configuration: &model.ClusterConfigurationEntity{
					Version:     1,
					KymaVersion: "1.20.0",
					KymaProfile: "production",
					Contract:    1,
				},
				Status: &model.ClusterStatusEntity{},
				Kubeconfig: &cluster.MockKubeconfigProvider{
					KubeconfigResult: string(kubeCfg),
				},
			},
			&workspace.Workspace{
				ComponentFile:           filepath.Join("test", "unittest-kyma", "components.yaml"),
				ResourceDir:             filepath.Join("test", "unittest-kyma", "resources"),
				InstallationResourceDir: filepath.Join("test", "unittest-kyma", "installation"),
			},
			&Options{})
		require.NoError(t, err)
		require.NotEmpty(t, manifests)
	})

}
