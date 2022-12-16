package rendering

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
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
		typedTestManifest := typedTestManifest()
		manifest, err := prepareTestManifest(&typedTestManifest)
		require.NoError(t, err)

		// when
		stub := NewChartProviderStub(manifest)
		provider := NewProviderWithFilters(stub)

		output, err := provider.RenderManifest(nil)
		require.NoError(t, err)

		// then
		require.Equal(t, manifest, output.Manifest)
	})

	t.Run("should render empty manifest if filter excluded every manifest", func(t *testing.T) {
		// given
		typedTestManifest := typedTestManifest()
		manifest, err := prepareTestManifest(&typedTestManifest)
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

	t.Run("should render manifest if filter included every manifest", func(t *testing.T) {
		// given
		typedTestManifest := typedTestManifest()
		manifest, err := prepareTestManifest(&typedTestManifest)
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
		outputTypedManifest, err := fromYamlToTypedObject(output.Manifest)
		require.NoError(t, err)

		require.Equal(t, typedTestManifest, outputTypedManifest)
	})

	t.Run("should fail if manifest is not a correct yaml", func(t *testing.T) {

	})

	t.Run("should fail if filter function failed", func(t *testing.T) {

	})
}

func typedTestManifest() corev1.ConfigMap {
	return corev1.ConfigMap{
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
		jsonserializer.DefaultMetaFactory, // jsonserializer.MetaFactory
		scheme.Scheme,                     // runtime.Scheme implements runtime.ObjectCreater
		scheme.Scheme,                     // runtime.Scheme implements runtime.ObjectTyper
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

func fromYamlToTypedObject(yamlManifest string) (corev1.ConfigMap, error) {
	decoded, err := runtime.Decode(unstructured.UnstructuredJSONScheme, []byte(yamlManifest))
	if err != nil {
		return corev1.ConfigMap{}, err
	}

	var tConfigMap corev1.ConfigMap
	unstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&decoded)
	if err != nil {
		return corev1.ConfigMap{}, err
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured, &tConfigMap)
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
}

func (cp *ChartProviderStub) WithFilter(filter chart.Filter) chart.Provider {
	return &ChartProviderStub{
		inputManifest: cp.inputManifest,
		filters:       append(cp.filters, filter),
	}
}

func (cp *ChartProviderStub) RenderCRD(version string) ([]*chart.Manifest, error) {
	return nil, nil
}

func (cp *ChartProviderStub) RenderManifest(component *chart.Component) (*chart.Manifest, error) {
	var err error
	manifest := cp.inputManifest

	for _, filter := range cp.filters {
		manifest, err = filter(manifest)
		if err != nil {
			return nil, err
		}
	}

	return &chart.Manifest{
		Manifest: manifest,
	}, nil
}

func (cp *ChartProviderStub) Configuration(component *chart.Component) (map[string]interface{}, error) {
	return nil, nil
}
