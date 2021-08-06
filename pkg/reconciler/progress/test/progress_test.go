package test

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/kubernetes"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/progress"

	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	e "github.com/kyma-incubator/reconciler/pkg/error"

	log "github.com/kyma-incubator/reconciler/pkg/logger"

	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestProgressTracker(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	logger, err := log.NewLogger(true)
	require.NoError(t, err)

	kubeClient, err := k8s.NewKubernetesClient(test.ReadKubeconfig(t), logger)
	require.NoError(t, err)

	clientSet, err := (&kubernetes.ClientBuilder{}).Build()
	require.NoError(t, err)

	manifest := readManifest(t)

	cleanup := func() {
		t.Log("Cleanup test resources")
		resources, err := kubeClient.Delete(manifest)
		require.NoError(t, err)
		require.Len(t, resources, 6)
		t.Logf("Removed %d test resources", len(resources))
	}
	cleanup()       //ensure relicts from previous test runs were removed
	defer cleanup() //cleanup after test is finished

	//install test resources
	t.Log("Deploying test resources")
	resources, err := kubeClient.Deploy(manifest)
	require.NoError(t, err)
	require.Len(t, resources, 6)
	t.Logf("Deployed %d test resources", len(resources))

	t.Run("Test progress tracking with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) //stop progress tracker after 1 sec
		defer cancel()

		pt, err := progress.NewProgressTracker(clientSet, logger,
			progress.Config{Interval: 1 * time.Second, Timeout: 1 * time.Minute})
		require.NoError(t, err)

		addWatchable(t, resources, pt)

		//check timeout happened within ~1 sec:
		startTime := time.Now()
		err = pt.Watch(ctx, progress.ReadyState)
		require.WithinDuration(t, startTime, time.Now(), 1250*time.Millisecond) //250msec as buffer to compensate overhead

		//err expected because a timeout occurred:
		require.Error(t, err)
		require.IsType(t, &e.ContextClosedError{}, err)
	})

	t.Run("Test progress tracking to state 'ready", func(t *testing.T) {
		// get progress tracker
		pt, err := progress.NewProgressTracker(clientSet, logger,
			progress.Config{Interval: 1 * time.Second, Timeout: 30 * time.Second})
		require.NoError(t, err)

		addWatchable(t, resources, pt)

		//depending on bandwidth, the installation should be finished latest after 30sec
		startTime := time.Now()
		require.NoError(t, pt.Watch(context.TODO(), progress.ReadyState))
		require.WithinDuration(t, startTime, time.Now(), 30*time.Second)
	})

	t.Run("Test progress tracking to state 'terminated'", func(t *testing.T) {
		cleanup() //delete resources

		//ensure progress returns error when checking for ready state of terminating resources
		go func(t *testing.T) {
			pt, err := progress.NewProgressTracker(clientSet, logger,
				progress.Config{Interval: 1 * time.Second, Timeout: 20 * time.Second})
			require.NoError(t, err)
			addWatchable(t, resources, pt)

			//Expect error as resources could not be watched properly when terminating/disappearing
			require.Error(t, pt.Watch(context.TODO(), progress.ReadyState))
			t.Log("Test successfully finished: checking for READY state failed with error")
		}(t)

		go func(t *testing.T) {
			pt, err := progress.NewProgressTracker(clientSet, logger,
				progress.Config{Interval: 1 * time.Second, Timeout: 15 * time.Second})
			require.NoError(t, err)
			addWatchable(t, resources, pt)

			//Expect NO error as resources are watched until they disappeared
			require.NoError(t, pt.Watch(context.TODO(), progress.TerminatedState))
			t.Log("Test successfully finished: checking for TERMINATED state finished without an error")
		}(t)

		time.Sleep(20 * time.Second)
	})
}

func addWatchable(t *testing.T, resources []*k8s.Resource, pt *progress.Tracker) {
	var cntWatchable int
	for _, resource := range resources {
		watchable, err := progress.NewWatchableResource(resource.Kind)
		if err == nil {
			t.Logf("Register watchable '%s'", resource)
			pt.AddResource(watchable, resource.Namespace, resource.Name)
			cntWatchable++
		}
	}
	require.Equal(t, 5, cntWatchable) //pod and a deployment has to be added as watchable
}

func readManifest(t *testing.T) string {
	manifest, err := ioutil.ReadFile(filepath.Join(".", "unittest.yaml"))
	require.NoError(t, err)
	return string(manifest)
}
