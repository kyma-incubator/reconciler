package chart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/internal/components"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	reconTest "github.com/kyma-incubator/reconciler/pkg/reconciler/test"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	kymaVersion        = "main"
	kymaNamespace      = "kyma-system"
	workspaceInHomeDir = "reconciliation-test" //TODO: use workspace in $HOME/.kyma per default - fix in WS-factory!
)

func TestProvider(t *testing.T) {
	test.IntegrationTest(t)

	log := logger.NewLogger(true)

	dirname, err := os.UserHomeDir()
	require.NoError(t, err)
	wsFactory, err := NewFactory(nil, filepath.Join(dirname, workspaceInHomeDir), log)
	require.NoError(t, err)

	prov, err := NewDefaultProvider(wsFactory, log)
	require.NoError(t, err)

	t.Parallel()

	t.Run("Render manifest", func(t *testing.T) {
		// kyma components
		ws, err := wsFactory.Get(kymaVersion)
		require.NoError(t, err)

		clist := componentList(t, filepath.Join(ws.InstallationResourceDir, "components.yaml"))

		for _, component := range clist {
			t.Logf("Rendering Kyma HELM component '%s'", component.name)
			manifest, err := prov.RenderManifest(component)
			require.NoError(t, err)
			require.Equal(t, component.name, manifest.Name)
			require.Equal(t, HelmChart, manifest.Type)
			require.NotEmpty(t, manifest.Manifest)
			require.NoError(t, yaml.Unmarshal([]byte(manifest.Manifest), make(map[string]interface{}))) //valid YAML
		}
	})

	t.Run("Render filtered manifest", func(t *testing.T) {
		// kyma components
		ws, err := wsFactory.Get(kymaVersion)
		require.NoError(t, err)

		provider, err := NewDefaultProvider(wsFactory, log)
		require.NoError(t, err)

		provider.WithFilter(func(manifest string) (string, error) { return "", nil })

		clist := componentList(t, filepath.Join(ws.InstallationResourceDir, "components.yaml"))

		for _, component := range clist {
			t.Logf("Rendering Kyma HELM component '%s'", component.name)
			manifest, err := provider.RenderManifest(component)
			require.NoError(t, err)
			require.Equal(t, component.name, manifest.Name)
			require.Equal(t, "", manifest.Manifest)
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

	tgz := NewComponentBuilder("main", "rma").
		WithURL("https://storage.googleapis.com/kyma-mps-dev-artifacts/rma-1.0.0.tgz").
		Build()

	git := NewComponentBuilder("main", "helloworld-chart").
		WithURL("https://github.com/srihas/helloworld-chart.git").
		Build()

	result = append(result, tgz, git)

	return result
}

func newComponent(comp components.Component) *Component {
	compBuilder := NewComponentBuilder(kymaVersion, comp.Name).
		WithConfiguration(reconTest.NewGlobalComponentConfiguration()).
		WithURL(comp.URL)

	if comp.Namespace == "" {
		compBuilder.WithNamespace(kymaNamespace)
	} else {
		compBuilder.WithNamespace(comp.Namespace)
	}

	return compBuilder.Build()
}
