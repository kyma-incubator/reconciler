package chart

import (
	reconTest "github.com/kyma-incubator/reconciler/pkg/reconciler/test"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"strings"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var (
	kymaVersion    = "main"
	kymaComponents = []string{"cluster-essentials", "istio-configuration@istio-system",
		"certificates@istio-system", "logging", "tracing", "kiali", "monitoring", "eventing", "ory", "api-gateway",
		"service-catalog", "service-catalog-addons", "rafter", "helm-broker", "cluster-users", "serverless",
		"application-connector@kyma-integration"}
)

func TestProvider(t *testing.T) {
	test.IntegrationTest(t)

	log, err := logger.NewLogger(true)
	require.NoError(t, err)

	wsFactory, err := workspace.NewFactory("test", log)
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
		for _, kymaComponent := range kymaComponents {
			component := newComponent(kymaComponent)
			t.Logf("Rendering Kyma HELM component '%s'", component)

			manifest, err := prov.RenderManifest(component)
			require.NoError(t, err)
			require.Equal(t, component.name, manifest.Name)
			require.Equal(t, HelmChart, manifest.Type)
			require.NotEmpty(t, manifest.Manifest)
			require.NoError(t, yaml.Unmarshal([]byte(manifest.Manifest), make(map[string]interface{})))
		}
	})

	t.Run("Render CRDs", func(t *testing.T) {
		crds, err := prov.RenderCRD(kymaVersion)
		require.NoError(t, err)
		require.NotEmpty(t, crds)
		require.Equal(t, crds[0].Type, CRD)
	})

}

func newComponent(comp string) *Component {
	compTokens := strings.Split(comp, "@")
	compBuilder := NewComponentBuilder(kymaVersion, compTokens[0]).
		WithConfiguration(reconTest.NewGlobalComponentConfiguration())

	if len(compTokens) < 2 {
		compBuilder.WithNamespace("kyma-system")
	} else {
		compBuilder.WithNamespace(compTokens[1])
	}

	return compBuilder.Build()
}
