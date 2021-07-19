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

	//find kubectl binary
	kubectl, err := Kubeclt()
	require.NoError(t, err)

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
	kubeClient, err := NewKubectlClient(kubectl, string(kubeconfig))

	t.Run("Deploy and delete resources", func(t *testing.T) {
		//deploy
		require.NoError(t, kubeClient.Deploy(string(manifest)))

		// 3 resource deployed
		resources, err := kubeClient.DeployedResources(string(manifest))
		require.NoError(t, err)
		require.Len(t, resources, 3)

		//delete
		require.NoError(t, kubeClient.Delete(string(manifest)))
	})

}
