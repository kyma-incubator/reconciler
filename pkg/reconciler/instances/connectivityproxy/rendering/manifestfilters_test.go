package rendering

import (
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	v1apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
)

func TestFilterByAnnotation(t *testing.T) {

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
		filter := NewFilterByAnnotation("skip")
		output, err := filter(input)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(output))
		require.Equal(t, "configMap1", output[0].GetName())
	})
}

func TestFilterByRelease(t *testing.T) {
	statefulSetName := "connectivity-proxy"
	statefulSetWithVersion24 := &v1apps.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   statefulSetName,
			Labels: map[string]string{"release": "2.4"},
		},
	}

	logger := zaptest.NewLogger(t).Sugar()

	statefulSetUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&statefulSetWithVersion24)
	require.NoError(t, err)

	input := []*unstructured.Unstructured{{Object: statefulSetUnstructured}}

	t.Run("should not filter out manifests if release differs", func(t *testing.T) {
		// when
		filter := NewFilterByRelease(logger, statefulSetName, "2.8")
		output, err := filter(input)

		// then
		require.NoError(t, err)
		require.Equal(t, input, output)
	})

	t.Run("should filter out all manifests is release doesn't differ", func(t *testing.T) {
		// when
		filter := NewFilterByRelease(logger, statefulSetName, "2.4")
		output, err := filter(input)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(output))
	})

	t.Run("should fail if StatefulSet not found", func(t *testing.T) {
		// given
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

		cmUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&configMap)
		require.NoError(t, err)

		input := []*unstructured.Unstructured{{Object: cmUnstructured}}

		// when
		filter := NewFilterByRelease(logger, statefulSetName, "2.4")
		output, err := filter(input)

		// then
		require.Error(t, err)
		require.Equal(t, []*unstructured.Unstructured{}, output)
	})

	t.Run("should fail if StatefulSet doesn't contain release label", func(t *testing.T) {
		// given
		statefulSetWithoutReleaseLabel := &v1apps.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: statefulSetName,
			},
		}

		statefulSetUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&statefulSetWithoutReleaseLabel)
		require.NoError(t, err)

		input := []*unstructured.Unstructured{{Object: statefulSetUnstructured}}

		// when
		filter := NewFilterByRelease(logger, statefulSetName, "2.4")
		output, err := filter(input)

		// then
		require.Error(t, err)
		require.Equal(t, []*unstructured.Unstructured{}, output)
	})
}
