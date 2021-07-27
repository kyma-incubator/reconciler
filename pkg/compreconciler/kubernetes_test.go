package compreconciler

import (
	"github.com/kyma-incubator/reconciler/pkg/compreconciler/types"
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
	kubeClient, err := newKubernetesClient(test.ReadKubeconfig(t))
	require.NoError(t, err)

	t.Run("Deploy and delete resources", func(t *testing.T) {
		manifest := readManifest(t)
		//deploy
		t.Log("Deploying test resources")
		_, resources, err := kubeClient.Deploy(manifest)
		require.NoError(t, err)
		//cleanup
		defer func() {
			t.Log("Cleanup test resources")
			require.NoError(t, kubeClient.Delete(manifest))
		}()

		require.NoError(t, err)
		require.ElementsMatch(t, []types.Metadata{
			{
				Kind:      "Deployment",
				Name:      "unittest-deployment",
				Namespace: "unittest",
			},
			{
				Kind:      "Pod",
				Name:      "unittest-pod",
				Namespace: "unittest",
			},
			{
				Kind:      "StatefulSet",
				Name:      "unittest-statefulset",
				Namespace: "unittest",
			},
			{
				Kind:      "DaemonSet",
				Name:      "unittest-daemonset",
				Namespace: "unittest",
			},
			{
				Kind:      "Job",
				Name:      "unittest-job",
				Namespace: "unittest",
			},
			{
				Kind:      "Namespace",
				Name:      "unittest",
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
