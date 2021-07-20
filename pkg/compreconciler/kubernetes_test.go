package compreconciler

import (
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"testing"
)

func TestKubernetesClient(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	//create client
	kubeClient, err := newKubernetesClient(readKubeconfig(t))
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
		require.ElementsMatch(t, []resource{
			{
				kind:      "Deployment",
				name:      "unittest-deployment",
				namespace: "unittest",
			},
			{
				kind:      "Pod",
				name:      "unittest-pod",
				namespace: "unittest",
			},
			{
				kind:      "StatefulSet",
				name:      "unittest-statefulset",
				namespace: "unittest",
			},
			{
				kind:      "DaemonSet",
				name:      "unittest-daemonset",
				namespace: "unittest",
			},
			{
				kind:      "Job",
				name:      "unittest-job",
				namespace: "unittest",
			},
			{
				kind:      "Namespace",
				name:      "unittest",
				namespace: "",
			},
		}, resources)
	})

	t.Run("Get Clientset", func(t *testing.T) {
		clientSet, err := kubeClient.Clientset()
		require.NoError(t, err)
		require.IsType(t, &kubernetes.Clientset{}, clientSet)
	})

}
