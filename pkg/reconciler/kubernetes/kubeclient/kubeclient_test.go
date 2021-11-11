package kubeclient

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	namespace    = "kubeclient-test"
	replacements = 3
)

func TestKubeclient(t *testing.T) {
	t.Parallel()

	kubeClient, err := NewKubeClient(test.ReadKubeconfig(t), logger.NewLogger(true))
	require.NoError(t, err)

	unstructs, err := ToUnstructured(readFile(t, filepath.Join("test", "manifest.yaml")), true)
	require.NoError(t, err)

	deleteNamespace(t, kubeClient, true)
	defer deleteNamespace(t, kubeClient, false)

	for _, unstruct := range unstructs {
		t.Run(fmt.Sprintf("Replacing %s", unstruct.GetName()), func(t *testing.T) {
			for i := 0; i <= replacements; i++ {
				if i > 0 {
					time.Sleep(1 * time.Second)
					t.Logf("Replacing %s the %d time", unstruct.GetKind(), i)
					unstruct.SetLabels(map[string]string{"replacements": fmt.Sprintf("%d", i)})
				}
				_, err = kubeClient.Apply(unstruct)
				require.NoError(t, err)
			}
			k8sResourceUnstruct, err := kubeClient.Get(unstruct.GetKind(), unstruct.GetName(), unstruct.GetNamespace())
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("%d", replacements), k8sResourceUnstruct.GetLabels()["replacements"])
		})
	}

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
