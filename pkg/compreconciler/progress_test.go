package compreconciler

import (
	"context"
	e "github.com/kyma-incubator/reconciler/pkg/error"
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

	//create Kubernetes client
	kubeClient, err := newKubernetesClient(test.ReadKubeconfig(t))
	require.NoError(t, err)

	//get client set
	clientSet, err := (&kubernetes.ClientBuilder{}).Build()
	require.NoError(t, err)

	//read resource manifest
	manifest := readManifest(t)

	//ensure old resources are deleted before running the test
	err = kubeClient.Delete(manifest)
	if err != nil {
		t.Log("Cleanup of old test resources failed (probably nothing to cleanup): test is continuing")
	}

	//install test resources
	t.Log("Deploying test resources")
	require.NoError(t, kubeClient.Deploy(manifest))
	defer func() {
		t.Log("Cleanup test resources")
		require.NoError(t, kubeClient.Delete(manifest))
	}()

	t.Run("Test progress tracking with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		pt, err := NewProgressTracker(ctx, clientSet, true,
			ProgressTrackerConfig{interval: 1 * time.Second, timeout: 2 * time.Second})
		require.NoError(t, err)

		addWatchable(t, manifest, pt, kubeClient)

		err = pt.Watch()
		require.Error(t, err)
		require.IsType(t, &e.ContextClosedError{}, err)
	})

	t.Run("Test progress tracking", func(t *testing.T) {
		// get progress tracker
		pt, err := NewProgressTracker(context.TODO(), clientSet, true,
			ProgressTrackerConfig{interval: 1 * time.Second, timeout: 35 * time.Second})
		require.NoError(t, err)

		addWatchable(t, manifest, pt, kubeClient)

		//depending on bandwidth, the installation should be finished latest after 30sec
		startTime := time.Now()
		require.NoError(t, pt.Watch())
		require.WithinDuration(t, startTime, time.Now(), 30*time.Second)
	})
}

func addWatchable(t *testing.T, manifest string, pt *ProgressTracker, kubeClient kubernetesClient) {
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
}
