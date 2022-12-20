package rendering

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"testing"
)

func TestChartProvider(t *testing.T) {
	t.Run("should render manifests if filters not provided", func(t *testing.T) {
		// given
		manifest, err := prepareTestManifest(typedTestManifest())
		require.NoError(t, err)

		// when
		stub := NewChartProviderStub(manifest)
		provider := NewProviderWithFilters(stub)

		output, err := provider.RenderManifest(nil)
		require.NoError(t, err)

		// then
		require.Equal(t, manifest, output.Manifest)
	})

	t.Run("should render empty string if filter excluded every manifest", func(t *testing.T) {
		// given
		manifest, err := prepareTestManifest(typedTestManifest())
		require.NoError(t, err)

		filterFunc := func([]*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{}, nil
		}

		// when
		stub := NewChartProviderStub(manifest)
		provider := NewProviderWithFilters(stub, filterFunc)

		output, err := provider.RenderManifest(nil)
		require.NoError(t, err)

		// then
		require.Equal(t, "", output.Manifest)
	})

	t.Run("should render manifests if filter included everything", func(t *testing.T) {
		// given
		typedManifest := typedTestManifest()
		manifest, err := prepareTestManifest(typedManifest)
		require.NoError(t, err)

		filterFunc := func(unstructured []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
			return unstructured, nil
		}

		// when
		stub := NewChartProviderStub(manifest)
		provider := NewProviderWithFilters(stub, filterFunc)

		output, err := provider.RenderManifest(nil)
		require.NoError(t, err)

		// then
		outputTypedManifest, err := fromJsonToTypedObject(output.Manifest)
		require.NoError(t, err)

		require.Equal(t, *typedManifest, outputTypedManifest)
	})

	t.Run("should fail if filter function failed", func(t *testing.T) {
		// given
		manifest, err := prepareTestManifest(typedTestManifest())
		require.NoError(t, err)

		filterFunc := func([]*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
			return []*unstructured.Unstructured{}, errors.New("some error")
		}
		// when
		stub := NewChartProviderStub(manifest)
		provider := NewProviderWithFilters(stub, filterFunc)

		output, err := provider.RenderManifest(nil)

		// then
		require.Error(t, err)
		require.Nil(t, output)
	})
}

func typedTestManifest() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configMap1",
			Namespace: "default",
		},

		Data: map[string]string{"foo": "bar"},
	}
}

func prepareTestManifest(object runtime.Object) (string, error) {

	serializer := jsonserializer.NewSerializerWithOptions(
		jsonserializer.DefaultMetaFactory,
		scheme.Scheme,
		scheme.Scheme,
		jsonserializer.SerializerOptions{
			Yaml:   true,
			Pretty: false,
			Strict: false,
		},
	)
	yaml, err := runtime.Encode(serializer, object)
	if err != nil {
		return "", err
	}

	return string(yaml), nil
}

func fromJsonToTypedObject(jsonManifest string) (corev1.ConfigMap, error) {
	decoded, err := runtime.Decode(unstructured.UnstructuredJSONScheme, []byte(jsonManifest))
	if err != nil {
		return corev1.ConfigMap{}, err
	}

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&decoded)
	if err != nil {
		return corev1.ConfigMap{}, err
	}

	var tConfigMap corev1.ConfigMap
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u, &tConfigMap)
	if err != nil {
		return corev1.ConfigMap{}, err
	}

	return tConfigMap, nil
}

func NewChartProviderStub(manifest string) chart.Provider {
	return &ChartProviderStub{
		inputManifest: manifest,
	}
}

type ChartProviderStub struct {
	filters       []chart.Filter
	inputManifest string
	lastError     error
}

func (cp *ChartProviderStub) WithFilter(filter chart.Filter) chart.Provider {
	return &ChartProviderStub{
		inputManifest: cp.inputManifest,
		filters:       append(cp.filters, filter),
	}
}

func (cp *ChartProviderStub) RenderCRD(_ string) ([]*chart.Manifest, error) {
	return nil, nil
}

func (cp *ChartProviderStub) RenderManifest(_ *chart.Component) (*chart.Manifest, error) {
	var err error
	manifest := cp.inputManifest

	for _, filter := range cp.filters {
		manifest, err = filter(manifest)
		if err != nil {
			cp.lastError = err
			return nil, err
		}
	}

	return &chart.Manifest{
		Manifest: manifest,
	}, nil
}

func (cp *ChartProviderStub) Configuration(_ *chart.Component) (map[string]interface{}, error) {
	return nil, nil
}
