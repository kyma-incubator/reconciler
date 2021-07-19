package compreconciler

import (
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestKubernetesClient(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	//read kubeconfig
	if os.Getenv("KUBECONFIG") == "" {
		require.FailNow(t, "Please set env-var 'KUBECONFIG' before running this test case")
	}
	kubeconfig, err := ioutil.ReadFile(os.Getenv("KUBECONFIG"))
	require.NoError(t, err)

	//read the manifest
	manifest, err := ioutil.ReadFile(filepath.Join("test", "deployment.yaml"))
	require.NoError(t, err)

	//create client
	kubeClient, err := NewClient(string(kubeconfig))

	t.Run("Deploy and delete resources", func(t *testing.T) {
		//deploy
		require.NoError(t, kubeClient.Deploy(string(manifest)))

		// 3 resource deployed
		resources, err := kubeClient.DeployedResources(string(manifest))
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
				kind:      "Namespace",
				name:      "unittest",
				namespace: "",
			},
		}, resources)

		//delete
		require.NoError(t, kubeClient.Delete(string(manifest)))
	})

	t.Run("Get Clientset", func(t *testing.T) {
		_, err := kubeClient.Clientset()
		require.NoError(t, err)
	})

}
