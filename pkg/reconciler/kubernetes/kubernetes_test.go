package kubernetes

import (
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"path/filepath"
	"testing"
)

func TestKubernetesClient(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	//create client
	kubeClient, err := NewKubernetesClient(test.ReadKubeconfig(t))
	require.NoError(t, err)

	t.Run("Deploy and delete resources", func(t *testing.T) {
		manifest := readManifest(t)
		//deploy
		t.Log("Deploying test resources")
		require.NoError(t, kubeClient.Deploy(manifest))
		//cleanup
		defer func() {
			t.Log("Cleanup test resources")
			require.NoError(t, kubeClient.Delete(manifest))
		}()

		// 6 resource deployed
		resources, err := kubeClient.DeployedResources(manifest)
		require.NoError(t, err)
		require.ElementsMatch(t, []Resource{
			{
				Kind:      "Deployment",
				Name:      "unittest-deployment",
				Namespace: "unittest-kubernetes",
			},
			{
				Kind:      "Pod",
				Name:      "unittest-pod",
				Namespace: "unittest-kubernetes",
			},
			{
				Kind:      "StatefulSet",
				Name:      "unittest-statefulset",
				Namespace: "unittest-kubernetes",
			},
			{
				Kind:      "DaemonSet",
				Name:      "unittest-daemonset",
				Namespace: "unittest-kubernetes",
			},
			{
				Kind:      "Job",
				Name:      "unittest-job",
				Namespace: "unittest-kubernetes",
			},
			{
				Kind:      "Namespace",
				Name:      "unittest-kubernetes",
				Namespace: "",
			},
		}, resources)
	})

	t.Run("Get Clientset", func(t *testing.T) {
		clientSet, err := kubeClient.Clientset()
		require.NoError(t, err)
		require.IsType(t, &kubernetes.Clientset{}, clientSet)
	})

}

func readManifest(t *testing.T) string {
	manifest, err := ioutil.ReadFile(filepath.Join("test", "unittest.yaml"))
	require.NoError(t, err)
	return string(manifest)
}
