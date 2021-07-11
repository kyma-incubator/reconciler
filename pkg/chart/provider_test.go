package chart

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
	"github.com/stretchr/testify/require"
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
			ComponentFile: filepath.Join(".", "test", "components.yaml"),
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

		expCompList, err := config.NewComponentList(filepath.Join(".", "test", "components-expected.yaml"))
		require.NoError(t, err)

		require.Equal(t, expCompList, compList)
	})
}
