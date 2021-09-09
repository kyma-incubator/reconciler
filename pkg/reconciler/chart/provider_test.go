package chart

import (
	"github.com/kyma-incubator/reconciler/internal/components"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/test"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	reconTest "github.com/kyma-incubator/reconciler/pkg/reconciler/test"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	kymaVersion   = "main"
	kymaNamespace = "kyma-system"
)

func TestProvider(t *testing.T) {
	test.IntegrationTest(t)

	log, err := logger.NewLogger(true)
	require.NoError(t, err)

	wsFactory, err := workspace.NewFactory(nil, "test", log)
	require.NoError(t, err)

	cleanupFct := func(t *testing.T) {
		require.NoError(t, wsFactory.Delete(kymaVersion))
	}

	//cleanup before test runs (to delete relicts of previous test executions) and after test is finished
	cleanupFct(t)
	defer cleanupFct(t)

	t.Parallel()

	prov, err := NewProvider(wsFactory, log)
	require.NoError(t, err)

	t.Run("Render manifest", func(t *testing.T) {
		ws, err := wsFactory.Get(kymaVersion)
		require.NoError(t, err)

		for _, component := range componentList(t, filepath.Join(ws.InstallationResourceDir, "components.yaml")) {
			t.Logf("Rendering Kyma HELM component '%s'", component.name)
			manifest, err := prov.RenderManifest(component)
			require.NoError(t, err)
			require.Equal(t, component.name, manifest.Name)
			require.Equal(t, HelmChart, manifest.Type)
			require.NotEmpty(t, manifest.Manifest)
			require.NoError(t, yaml.Unmarshal([]byte(manifest.Manifest), make(map[string]interface{}))) //valid YAML
		}
	})

	t.Run("Render CRDs", func(t *testing.T) {
		crds, err := prov.RenderCRD(kymaVersion)
		require.NoError(t, err)
		require.NotEmpty(t, crds)
		require.Equal(t, crds[0].Type, CRD)
	})

}

func componentList(t *testing.T, compListFile string) []*Component {
	compList, err := components.NewComponentList(compListFile)
	require.NoError(t, err)

	var result []*Component
	for _, comp := range compList.Prerequisites {
		result = append(result, newComponent(comp))
	}
	for _, comp := range compList.Components {
		result = append(result, newComponent(comp))
	}

	return result
}

func newComponent(comp components.Component) *Component {
	compBuilder := NewComponentBuilder(kymaVersion, comp.Name).
		WithConfiguration(reconTest.NewGlobalComponentConfiguration())

	if comp.Namespace == "" {
		compBuilder.WithNamespace(kymaNamespace)
	} else {
		compBuilder.WithNamespace(comp.Namespace)
	}

	return compBuilder.Build()
}
