package rendering

import (
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

func TestFilterOutAnnotatedManifests(t *testing.T) {

	configMap := corev1.ConfigMap{
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

	configMapToFilterOut := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "configMap2",
			Namespace:   "default",
			Annotations: map[string]string{"skip": "true"},
		},

		Data: map[string]string{"foo": "bar"},
	}

	t.Run("should filter out manifests containing an annotation", func(t *testing.T) {
		// given
		cmUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&configMap)
		require.NoError(t, err)

		cmToFilterOutUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&configMapToFilterOut)
		require.NoError(t, err)

		input := []*unstructured.Unstructured{
			{
				Object: cmUnstructured,
			}, {
				Object: cmToFilterOutUnstructured,
			}}

		// when
		filter := NewFilterOutAnnotatedManifests("skip")
		output, err := filter(input)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(output))
		require.Equal(t, "configMap1", output[0].GetName())
	})
}
