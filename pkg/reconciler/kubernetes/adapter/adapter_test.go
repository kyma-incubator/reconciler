package adapter

import (
	"context"
	kubernetes2 "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"io/ioutil"
	"path/filepath"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
)

var expectedResources = []*kubernetes2.Resource{
	{
		Kind:      "Deployment",
		Name:      "unittest-deployment",
		Namespace: "default",
	},
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
}

func TestKubernetesClient(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	//create client
	kubeClient, err := NewKubernetesClient(test.ReadKubeconfig(t), log.NewOptionalLogger(true), nil)
	require.NoError(t, err)

	t.Run("Deploy and delete resources", func(t *testing.T) {
		manifest := readManifest(t)

		//deploy
		t.Log("Deploying test resources")
		deployedResources, err := kubeClient.Deploy(context.TODO(), manifest)
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResources, deployedResources)

		//delete (at the end of the test)
		t.Log("Cleanup test resources")
		deletedResources, err := kubeClient.Delete(context.TODO(), manifest)
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResources, deletedResources)

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
