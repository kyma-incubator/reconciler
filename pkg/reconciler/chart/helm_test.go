package chart

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"

	"github.com/stretchr/testify/require"
)

const (
	profileName   = "profile"
	componentName = "component-1"
)

var chartDir = filepath.Join("test", "unittest-kyma", "resources")

func TestHelm(t *testing.T) {
	logger := log.NewLogger(true)

	t.Run("Get chart configuration without profile", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.profileConfiguration(loadHelmChart(t, component), "", false)
		require.NoError(t, err)

		var expected map[string]interface{}
		err = json.Unmarshal([]byte(`{
			"config": {
				"key1": "value1 from values.yaml",
				"key2": "value2 from values.yaml"
			},
			"showKey2": false
		}`), &expected)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("Get chart configuration with profile and without values", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			WithProfile(profileName).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.profileConfiguration(loadHelmChart(t, component), profileName, false)
		require.NoError(t, err)

		var expected map[string]interface{}
		err = json.Unmarshal([]byte(`{
			"config": {
				"key1": "value1 from profile.yaml",
				"key2": "value2 from profile.yaml"
			},
			"profile": true
		}`), &expected)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("Get chart configuration with profile and with values", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			WithProfile(profileName).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.profileConfiguration(loadHelmChart(t, component), profileName, true)
		require.NoError(t, err)

		var expected map[string]interface{}
		err = json.Unmarshal([]byte(`{
			"config": {
				"key1": "value1 from profile.yaml",
				"key2": "value2 from profile.yaml"
			},
			"profile": true,
			"showKey2": false
		}`), &expected)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("Merge chart configuration with empty component configuration", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			WithProfile(profileName).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.mergeChartConfiguration(loadHelmChart(t, component), component, false)
		require.NoError(t, err)
		var expected map[string]interface{}
		err = json.Unmarshal([]byte(`{
			"config": {
				"key1": "value1 from profile.yaml",
				"key2": "value2 from profile.yaml"
			},
			"profile": true
		}`), &expected)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("Merge chart configuration with component configuration", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			WithProfile(profileName).
			WithConfiguration(map[string]interface{}{
				"config.key2":           "value2 from component",
				"component.config.key1": "123.4",
				"component.config.key2": "true",
			}).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.mergeChartConfiguration(loadHelmChart(t, component), component, false)
		require.NoError(t, err)

		var expected map[string]interface{}
		err = json.Unmarshal([]byte(`{
			"config": {
				"key1": "value1 from profile.yaml",
				"key2": "value2 from component"
			},
			"profile": true,
			"component": {
				"config": {
					"key1": "123.4",
					"key2": "true"
				}
			}
		}`), &expected)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("Render template", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			WithProfile(profileName).
			WithConfiguration(map[string]interface{}{
				"config.key2": "value2 from component",
				"showKey2":    true,
			}).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.Render(component)
		require.NoError(t, err)
		gotAsMap := make(map[string]interface{})
		require.NoError(t, yaml.Unmarshal([]byte(got), &gotAsMap)) //use for equality check (avoids whitespace diffs)

		expected, err := os.ReadFile(filepath.Join(chartDir, componentName, "configmap-expected.yaml"))
		require.NoError(t, err)
		expectedAsMap := make(map[string]interface{})
		require.NoError(t, yaml.Unmarshal(expected, &expectedAsMap)) //use for equality check (avoids whitespace diffs)

		require.Equal(t, expectedAsMap, gotAsMap)
	})
}

func loadHelmChart(t *testing.T, component *Component) *chart.Chart {
	helmChart, err := loader.Load(filepath.Join(chartDir, component.name))
	require.NoError(t, err)
	return helmChart
}
