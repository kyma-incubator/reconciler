package internal

import (
	"context"
	"fmt"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
)

func TestSetNamespaceIfScoped(t *testing.T) {
	t.Run("Should set namespace if scoped", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		setNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: true,
		})
		require.Equal(t, "test", u.GetNamespace())
	})

	t.Run("Ignore namespace if not scoped", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetNamespace("")
		setNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: false,
		})
		require.Equal(t, "", u.GetNamespace())

		u.SetNamespace("initial")
		setNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: false,
		})
		require.Equal(t, "initial", u.GetNamespace())

	})
}

const (
	namespace          = "kubeclient-test"
	containerBaseImage = "alpine"
)

func TestPatchReplace(t *testing.T) {
	test.IntegrationTest(t)

	t.Parallel()

	kubeClient, err := NewKubeClient(test.ReadKubeconfig(t), logger.NewLogger(true))
	require.NoError(t, err)

	deleteNamespace(t, kubeClient, true)
	defer deleteNamespace(t, kubeClient, false)

	for idxManifest, manifest := range []string{
		"manifest-before.yaml",
		"manifest-after.yaml",
		"manifest-before.yaml",
		"manifest-after.yaml"} {
		unstructs, err := ToUnstructured(readFile(t, filepath.Join("test", manifest)), true)
		require.NoError(t, err)

		var expectedImage = containerBaseImage
		if idxManifest%2 == 1 { //each second update updates the image from 'alpine' to 'alpine:3.14'
			expectedImage = fmt.Sprintf("%s:3.14", expectedImage)
		}

		for idxUnstruct, unstruct := range unstructs {
			t.Run(
				fmt.Sprintf("Applying %s", unstruct.GetName()),
				newTestFunc(kubeClient, unstruct, fmt.Sprintf("%d_%d", idxManifest, idxUnstruct), expectedImage))
		}
	}

}

func newTestFunc(kubeClient *KubeClient, unstruct *unstructured.Unstructured, label, expectedImage string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Logf("Applying %s and setting label %s", unstruct.GetKind(), label)
		unstruct.SetLabels(map[string]string{"applied": label})

		_, err := kubeClient.Apply(unstruct)
		require.NoError(t, err)

		k8sResourceUnstruct, err := kubeClient.Get(unstruct.GetKind(), unstruct.GetName(), unstruct.GetNamespace())
		require.NoError(t, err)

		//verify label
		require.Equal(t, label, k8sResourceUnstruct.GetLabels()["applied"])

		//verify image
		switch strings.ToLower(k8sResourceUnstruct.GetKind()) {
		case "statefulset":
			var statefulSet v1.StatefulSet
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(unstruct.UnstructuredContent(), &statefulSet)
			require.NoError(t, err)
			for _, container := range statefulSet.Spec.Template.Spec.Containers {
				if requireImageName(t, container, expectedImage) {
					return
				}
			}
		case "deployment":
			var deployment v1.Deployment
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(unstruct.UnstructuredContent(), &deployment)
			require.NoError(t, err)
			for _, container := range deployment.Spec.Template.Spec.Containers {
				if requireImageName(t, container, expectedImage) {
					return
				}
			}
		}
	}
}

func requireImageName(t *testing.T, container corev1.Container, expectedImage string) bool {
	if strings.HasPrefix(container.Image, containerBaseImage) {
		t.Logf("Found container image '%s'", expectedImage)
		require.Equal(t, expectedImage, container.Image)
		return true
	}
	return false
}

func deleteNamespace(t *testing.T, kubeClient *KubeClient, ignoreError bool) {
	clientSet, err := kubeClient.GetClientSet()
	require.NoError(t, err)
	err = clientSet.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
	if !ignoreError {
		require.NoError(t, err)
	}
	ticker := time.NewTicker(1 * time.Second)
	timeout := time.After(15 * time.Second)
	select {
	case <-ticker.C:
		_, err := clientSet.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			t.Logf("Namespace '%s' deleted", namespace)
			break
		}
	case <-timeout:
		t.Logf("Timeout reached: namespace '%s' not deleted within expected time range", namespace)
		break
	}
}

func readFile(t *testing.T, file string) []byte {
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	return data
}
