package service

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallLookup(t *testing.T) {
	unstructuredName := "unstructuredname"
	manifestString := string(readFile(t, filepath.Join("test", "manifest.yaml")))

	installService := NewInstall(logger.NewLogger(true))

	t.Run("should find an unstructured", func(t *testing.T) {
		// given
		condition := func(unstructured *unstructured.Unstructured) bool {
			return unstructured != nil && unstructured.GetName() == unstructuredName
		}
		task := &reconciler.Task{Component: "notCRDs"}
		chartProvider := &chartmocks.Provider{}
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{Manifest: manifestString}, nil)

		// when
		unstr, err := installService.Lookup(condition, chartProvider, task)

		// then
		require.NoError(t, err)
		require.NotNil(t, unstr)
		assert.Equal(t, unstructuredName, unstr.GetName())
	})

	t.Run("should not find an unstructured", func(t *testing.T) {
		// given
		condition := func(unstructured *unstructured.Unstructured) bool {
			return unstructured != nil && unstructured.GetName() == "sometotallydifferentname"
		}
		task := &reconciler.Task{Component: "notCRDs"}
		chartProvider := &chartmocks.Provider{}
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{Manifest: manifestString}, nil)

		// when
		unstr, err := installService.Lookup(condition, chartProvider, task)

		// then
		require.NoError(t, err)
		assert.Nil(t, unstr)
	})

	t.Run("should fail if applied to CRDs component", func(t *testing.T) {
		// given
		condition := func(unstructured *unstructured.Unstructured) bool {
			return unstructured != nil && unstructured.GetName() == unstructuredName
		}
		task := &reconciler.Task{Component: model.CRDComponent}
		chartProvider := &chartmocks.Provider{}
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{Manifest: manifestString}, nil)

		// when
		_, err := installService.Lookup(condition, chartProvider, task)

		// then
		require.Error(t, err)
	})

	t.Run("should fail if failed to render manifest", func(t *testing.T) {
		// given
		condition := func(unstructured *unstructured.Unstructured) bool {
			return unstructured != nil && unstructured.GetName() == unstructuredName
		}
		task := &reconciler.Task{Component: "notCRDs"}
		chartProvider := &chartmocks.Provider{}
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(nil, errors.New("some error"))

		// when
		_, err := installService.Lookup(condition, chartProvider, task)

		// then
		require.Error(t, err)
	})
}

func readFile(t *testing.T, file string) []byte {
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	return data
}
