package progress

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	e "github.com/kyma-incubator/reconciler/pkg/error"
	"github.com/kyma-incubator/reconciler/pkg/kubernetes"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestProgressTracker(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	logger := log.NewOptionalLogger(true)

	//create Kubernetes client
	kubeClient, err := k8s.NewKubernetesClient(test.ReadKubeconfig(t), logger)
	require.NoError(t, err)

	//get client set
	clientSet, err := (&kubernetes.ClientBuilder{}).Build()
	require.NoError(t, err)

	//read resource manifest
	manifest := readManifest(t)

	cleanup := func() {
		t.Log("Cleanup test resources")
		if err := kubeClient.Delete(manifest); err != nil {
			t.Log("Cleanup of test resources failed (probably nothing to cleanup): test is continuing")
		}
	}
	cleanup()       //ensure relicts from previous test runs were removed
	defer cleanup() //cleanup after test is finished

	//install test resources
	t.Log("Deploying test resources")
	resources, err := kubeClient.Deploy(manifest)
	require.NoError(t, err)
	require.Len(t, resources, 6)

	t.Run("Test progress tracking with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) //stop progress tracker after 1 sec
		defer cancel()

		pt, err := NewProgressTracker(ctx, clientSet, logger,
			Config{Interval: 1 * time.Second, Timeout: 1 * time.Minute})
		require.NoError(t, err)

		addWatchable(t, resources, pt)

		//check timeout happened within ~1 sec:
		startTime := time.Now()
		err = pt.Watch()
		require.WithinDuration(t, startTime, time.Now(), 1250*time.Millisecond) //250msec as buffer to compensate overhead

		//err expected because a timeout occurred:
		require.Error(t, err)
		require.IsType(t, &e.ContextClosedError{}, err)
	})

	t.Run("Test progress tracking", func(t *testing.T) {
		// get progress tracker
		pt, err := NewProgressTracker(context.TODO(), clientSet, logger,
			Config{Interval: 1 * time.Second, Timeout: 35 * time.Second})
		require.NoError(t, err)

		addWatchable(t, resources, pt)

		//depending on bandwidth, the installation should be finished latest after 30sec
		startTime := time.Now()
		require.NoError(t, pt.Watch())
		require.WithinDuration(t, startTime, time.Now(), 30*time.Second)
	})

	t.Run("Test progress tracking when state is terminating", func(t *testing.T) {
		cleanup() //delete resources

		// get progress tracker
		pt, err := NewProgressTracker(context.TODO(), clientSet, logger,
			Config{Interval: 1 * time.Second, Timeout: 10 * time.Second})
		require.NoError(t, err)

		addWatchable(t, resources, pt)

		//Expect error as resources could not be watched properly when terminating/disappearing
		require.Error(t, pt.Watch())
	})
}

func addWatchable(t *testing.T, resources []*k8s.Resource, pt *Tracker) {
	var cntWatchable int
	for _, resource := range resources {
		watchable, err := NewWatchableResource(resource.Kind)
		if err == nil {
			t.Logf("Register watchable '%s'", resource)
			pt.AddResource(watchable, resource.Namespace, resource.Name)
			cntWatchable++
		}
	}
	require.Equal(t, 5, cntWatchable) //pod and a deployment has to be added as watchable
}

func readManifest(t *testing.T) string {
	manifest, err := ioutil.ReadFile(filepath.Join("test", "unittest.yaml"))
	require.NoError(t, err)
	return string(manifest)
}
