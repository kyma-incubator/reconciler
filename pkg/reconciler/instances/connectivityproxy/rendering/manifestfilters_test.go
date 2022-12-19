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

func TestSkipReinstallingCurrentRelease(t *testing.T) {
	statefulSetName := "connectivity-proxy"
	statefulSetWithVersion24 := &v1apps.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   statefulSetName,
			Labels: map[string]string{"release": "2.4.0"},
		},
	}

	logger := zaptest.NewLogger(t).Sugar()
	input := getInput(t, &statefulSetWithVersion24)

	t.Run("should not filter out manifests if release to be installed in newer", func(t *testing.T) {
		// when
		filter := NewSkipReinstallingCurrentRelease(logger, statefulSetName, "2.3.0")
		output, err := filter(input)

		// then
		require.NoError(t, err)
		require.Equal(t, input, output)
	})

	t.Run("should filter out all manifests if release to be installed is not greater than current one", func(t *testing.T) {
		// when
		filter := NewSkipReinstallingCurrentRelease(logger, statefulSetName, "2.4.0")
		output, err := filter(input)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(output))

		// when
		filter = NewSkipReinstallingCurrentRelease(logger, statefulSetName, "2.5.0")
		output, err = filter(input)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(output))
	})
}

func TestSkipReinstallingCurrentReleaseErrors(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	statefulSetName := "connectivity-proxy"

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

		input := getInput(t, &configMap)

		// when
		filter := NewSkipReinstallingCurrentRelease(logger, statefulSetName, "2.4.0")
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

		input := getInput(t, &statefulSetWithoutReleaseLabel)

		// when
		filter := NewSkipReinstallingCurrentRelease(logger, statefulSetName, "2.4.0")
		output, err := filter(input)

		// then
		require.Error(t, err)
		require.Equal(t, []*unstructured.Unstructured{}, output)
	})

	t.Run("should fail if the version has incorrect format", func(t *testing.T) {
		// given
		statefulSetWithIncorrectReleaseLabel := &v1apps.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   statefulSetName,
				Labels: map[string]string{"release": "2.4"},
			},
		}

		input := getInput(t, &statefulSetWithIncorrectReleaseLabel)

		// when
		filter := NewSkipReinstallingCurrentRelease(logger, statefulSetName, "2.4.0")
		output, err := filter(input)

		// then
		require.Error(t, err)
		require.Equal(t, []*unstructured.Unstructured{}, output)

		// when
		filter = NewSkipReinstallingCurrentRelease(logger, statefulSetName, "2.4")
		output, err = filter(input)

		// then
		require.Error(t, err)
		require.Equal(t, []*unstructured.Unstructured{}, output)
	})
}

func getInput(t *testing.T, obj interface{}) []*unstructured.Unstructured {
	t.Helper()

	var uns []*unstructured.Unstructured

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	require.NoError(t, err)

	uns = append(uns, &unstructured.Unstructured{Object: u})

	return uns
}
