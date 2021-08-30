package chart

import (
	"encoding/json"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	profileName   = "profile"
	componentName = "component-1"
)

var chartDir = filepath.Join("test", "unittest-kyma", "resources")

func TestHelm(t *testing.T) {
	logger, err := log.NewLogger(true)
	require.NoError(t, err)

	t.Run("Get chart configuration without profile", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.chartConfiguration(loadHelmChart(t, component), "")
		require.NoError(t, err)

		var expected map[string]interface{}
		err = json.Unmarshal([]byte(`{
			"config": {
				"key1": "value1 from values.yaml",
				"key2": "value2 from values.yaml"
			}
		}`), &expected)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})

	t.Run("Get chart configuration with profile", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			WithProfile(profileName).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.chartConfiguration(loadHelmChart(t, component), profileName)
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

	t.Run("Merge chart configuration with empty component configuration", func(t *testing.T) {
		component := NewComponentBuilder("main", componentName).
			WithNamespace("testNamespace").
			WithProfile(profileName).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.mergeChartConfiguration(loadHelmChart(t, component), component)
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
				"component.config.key1": 123.4,
				"component.config.key2": true,
			}).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.mergeChartConfiguration(loadHelmChart(t, component), component)
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
					"key1": 123.4,
					"key2": true
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
			}).
			Build()

		helm, err := NewHelmClient(chartDir, logger)
		require.NoError(t, err)

		got, err := helm.Render(component)
		require.NoError(t, err)

		expected, err := ioutil.ReadFile(filepath.Join(chartDir, componentName, "configmap-expected.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(expected), got)
	})
}

func loadHelmChart(t *testing.T, component *Component) *chart.Chart {
	helmChart, err := loader.Load(filepath.Join(chartDir, component.name))
	require.NoError(t, err)
	return helmChart
}
