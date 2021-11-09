package adapter

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
)

var expectedResourcesWithoutNs = []*k8s.Resource{
	{
		Kind:      "Deployment",
		Name:      "unittest-deployment",
		Namespace: "default",
	},
}

var expectedResourcesWithNs = []*k8s.Resource{
	{
		Kind:      "Namespace",
		Name:      "unittest-adapter",
		Namespace: "",
	},
	{
		Kind:      "Deployment",
		Name:      "unittest-deployment",
		Namespace: "unittest-adapter",
	},
	{
		Kind:      "StatefulSet",
		Name:      "unittest-statefulset",
		Namespace: "unittest-adapter",
	},
	{
		Kind:      "DaemonSet",
		Name:      "unittest-daemonset",
		Namespace: "unittest-adapter",
	},
	{
		Kind:      "Job",
		Name:      "unittest-job",
		Namespace: "unittest-adapter",
	},
}

func TestKubernetesClient(t *testing.T) {
	test.IntegrationTest(t)

	//create client
	kubeClient, err := NewKubernetesClient(test.ReadKubeconfig(t), log.NewLogger(true), &Config{
		ProgressInterval: 1 * time.Second,
		ProgressTimeout:  1 * time.Minute,
	})
	require.NoError(t, err)

	t.Run("Deploy and delete resources with namespace", func(t *testing.T) {
		manifestWithNs := readManifest(t, "unittest-with-namespace.yaml")

		//deploy
		t.Log("Deploying test resources")
		deployedResources, err := kubeClient.Deploy(context.TODO(), manifestWithNs, "unittest-adapter")
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResourcesWithNs, deployedResources)

		//delete (at the end of the test)
		t.Log("Cleanup test resources")
		deletedResources, err := kubeClient.Delete(context.TODO(), manifestWithNs, "unittest-adapter")
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResourcesWithNs, deletedResources)
	})

	t.Run("Deploy and delete resources without namespace", func(t *testing.T) {
		manifestWithNs := readManifest(t, "unittest-without-namespace.yaml")

		//deploy
		t.Log("Deploying test resources")
		deployedResources, err := kubeClient.Deploy(context.TODO(), manifestWithNs, "")
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResourcesWithoutNs, deployedResources)

		//delete (at the end of the test)
		t.Log("Cleanup test resources")
		deletedResources, err := kubeClient.Delete(context.TODO(), manifestWithNs, "")
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResourcesWithoutNs, deletedResources)
	})

	t.Run("Get Clientset", func(t *testing.T) {
		clientSet, err := kubeClient.Clientset()
		require.NoError(t, err)
		require.IsType(t, &kubernetes.Clientset{}, clientSet)
	})

}

func readManifest(t *testing.T, fileName string) string {
	manifest, err := ioutil.ReadFile(filepath.Join("test", fileName))
	require.NoError(t, err)
	return string(manifest)
}
