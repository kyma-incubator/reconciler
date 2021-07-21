package compreconciler

import (
	"github.com/kyma-incubator/reconciler/pkg/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestProgressTracker(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	//deploy different resources
	manifest := readManifest(t)
	kubeClient, err := newKubernetesClient(readKubeconfig(t))
	require.NoError(t, err)

	//ensure old resources are deleted before running the test
	err = kubeClient.Delete(manifest)
	if err != nil {
		t.Log("Cleanup of old test resources failed (probably nothing to cleanup): test is continuing")
	}

	//install resources
	t.Log("Deploying test resources")
	require.NoError(t, kubeClient.Deploy(manifest))
	defer func() {
		t.Log("Cleanup test resources")
		require.NoError(t, kubeClient.Delete(manifest))
	}()

	// get progress tracker
	clientSet, err := (&kubernetes.ClientBuilder{}).Build()
	require.NoError(t, err)
	//depending on the network bandwidth, the timeout could be too low
	pt, err := NewProgressTracker(clientSet, true,
		ProgressTrackerConfig{interval: 1 * time.Second, timeout: 20 * time.Second})
	require.NoError(t, err)

	//watch created resources
	resources, err := kubeClient.DeployedResources(manifest)
	require.NoError(t, err)

	var cntWatchable int
	for _, resource := range resources {
		watchable, err := NewWatchableResource(resource.kind)
		if err == nil {
			pt.AddResource(watchable, resource.namespace, resource.name)
			cntWatchable++
		}
	}
	require.Equal(t, 5, cntWatchable) //pod and a deployment has to be added as watchable

	//depending on bandwidth, the installation should be finished latest after 15sec
	startTime := time.Now()
	require.NoError(t, pt.Watch())
	require.WithinDuration(t, startTime, time.Now(), 15*time.Second)
}
