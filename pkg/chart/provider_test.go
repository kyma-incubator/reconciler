package chart

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var (
	componentListFile         string = filepath.Join("test", "unittest-complist.yaml")
	componentListExpectedFile string = filepath.Join("test", "unittest-complist-expected.yaml")
)

func TestProvider(t *testing.T) {
	t.Parallel()

	wsFactory := &workspace.Factory{
		StorageDir: "./test",
	}
	prov, err := NewProvider(wsFactory, true)
	require.NoError(t, err)

	t.Run("Convert dot-notated configuration keys to a nested map", func(t *testing.T) {
		got := prov.nestedConfMap("this.is.a.test", "the test value")
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

	t.Run("Test overrides processing", func(t *testing.T) {
		builder, err := prov.overrides([]*Component{
			{
				name: "test-component",
				configuration: map[string]interface{}{
					"test.key1.subkey1": "test value 1",
					"test.key1.subkey2": "test value 2",
					"test.key2.subkey1": "test value 3"},
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
		}, []*Component{
			{
				name:      "component-2",
				namespace: "differentns-component-2",
			},
			{
				name:      "component-3",
				namespace: "differentns-component-3",
			},
		})
		require.NoError(t, err)

		expCompList, err := config.NewComponentList(componentListExpectedFile)
		require.NoError(t, err)

		require.Equal(t, expCompList, compList)
	})

	t.Run("Test render manifest", func(t *testing.T) {
		if !test.RunExpensiveTests() {
			return
		}

		if !file.Exists(os.Getenv("KUBECONFIG")) {
			require.FailNow(t, "Please set env-var KUBECONFIG before executing this test case")
		}

		kubeCfg, err := ioutil.ReadFile(os.Getenv("KUBECONFIG"))
		require.NoError(t, err)
		compSet := NewComponentSet(string(kubeCfg), "2.0.0", "testProfile", []*Component{
			{
				name:      "component-1",
				namespace: "different-namespace",
				configuration: map[string]interface{}{
					"dummy.config": "overwritten by unittest",
				},
			},
		})

		manifests, err := prov.renderManifests(
			compSet,
			&workspace.Workspace{
				ComponentFile:           filepath.Join("test", "unittest-kyma", "components.yaml"),
				ResourceDir:             filepath.Join("test", "unittest-kyma", "resources"),
				InstallationResourceDir: filepath.Join("test", "unittest-kyma", "installation"),
			},
			&Options{})
		require.NoError(t, err)

		for _, manifest := range manifests {
			var exp, got interface{}
			if manifest.Type == components.CRD {
				exp = expected(t, filepath.Join("test", "unittest-kyma", "installation", "crds", "component-1", "crd.yaml"))
				got = map[string]interface{}{}
				require.NoError(t, yaml.Unmarshal([]byte(manifest.Manifest), got))
			} else {
				exp = expected(t, filepath.Join("test", "unittest-kyma", "resources", "component-1", "configmap-expected.yaml"))
				got = map[string]interface{}{}
				require.NoError(t, yaml.Unmarshal([]byte(manifest.Manifest), got))
			}
			require.Equal(t, exp, got)
		}
	})

}

func expected(t *testing.T, file string) map[string]interface{} {
	data, err := ioutil.ReadFile(file)
	require.NoError(t, err)
	expected := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal(data, expected))
	return expected
}
