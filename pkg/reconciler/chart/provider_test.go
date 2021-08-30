package chart

import (
	"github.com/kyma-incubator/reconciler/pkg/test"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestProvider(t *testing.T) {
	test.IntegrationTest(t)

	var (
		kymaVersion   = "main"
		kymaComponent = "cluster-users"
	)

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
		component := NewComponentBuilder(kymaVersion, kymaComponent).
			WithNamespace("kyma-system").
			Build()

		manifest, err := prov.RenderManifest(component)
		require.NoError(t, err)
		require.Equal(t, kymaComponent, manifest.Name)
		require.Equal(t, HelmChart, manifest.Type)
		require.True(t, len(manifest.Manifest) > 1000)
		require.NoError(t, yaml.Unmarshal([]byte(manifest.Manifest), make(map[string]interface{})))
	})

	t.Run("Render CRDs", func(t *testing.T) {
		crds, err := prov.RenderCRD(kymaVersion)
		require.NoError(t, err)
		require.NotEmpty(t, crds)
		require.Equal(t, crds[0].Type, CRD)
	})

}
