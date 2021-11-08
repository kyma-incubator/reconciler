package progress

import (
	"context"
	e "github.com/kyma-incubator/reconciler/pkg/error"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"go.uber.org/zap"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"strings"

	"path/filepath"
	"testing"
	"time"

	log "github.com/kyma-incubator/reconciler/pkg/logger"

	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestProgressTracker(t *testing.T) {
	test.IntegrationTest(t)

	logger := log.NewLogger(true)

	kubeClient, err := kubeclient.NewKubeClient(test.ReadKubeconfig(t), zap.NewNop().Sugar())
	require.NoError(t, err)

	clientSet, err := kubeClient.GetClientSet()
	require.NoError(t, err)

	resources := readManifest(t, "all.yaml")
	require.Len(t, resources, 5)

	cleanup := func() {
		t.Log("Cleanup test resources")
		for _, resource := range resources {
			deletedResource, err := kubeClient.DeleteResourceByKindAndNameAndNamespace(
				resource.GetKind(), resource.GetName(), resource.GetNamespace(), metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				t.Fatalf("Failed to delete resource: %s", err)
			}
			t.Logf("Removed test resource '%s", deletedResource)
		}
	}
	cleanup()       //ensure relicts from previous test runs were removed
	defer cleanup() //cleanup after test is finished

	//install test resources
	t.Log("Creating test resources")
	for _, resource := range resources {
		deployedResource, err := kubeClient.Apply(resource)
		require.NoError(t, err)
		t.Logf("Deployed test resource '%s", deployedResource)
	}

	t.Run("Test progress tracking with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) //stop progress tracker after 1 sec
		defer cancel()

		pt, err := NewProgressTracker(clientSet, logger,
			Config{Interval: 1 * time.Second, Timeout: 1 * time.Minute})
		require.NoError(t, err)

		addWatchable(t, resources, pt)

		//check timeout happened within ~1 sec:
		startTime := time.Now()
		err = pt.Watch(ctx, ReadyState)
		require.WithinDuration(t, startTime, time.Now(), 1250*time.Millisecond) //250msec as buffer to compensate overhead

		//err expected because a timeout occurred:
		require.Error(t, err)
		require.IsType(t, &e.ContextClosedError{}, err)
	})

	t.Run("Test progress tracking to state 'ready'", func(t *testing.T) {
		// get progress tracker
		pt, err := NewProgressTracker(clientSet, logger,
			Config{Interval: 1 * time.Second, Timeout: 1 * time.Minute})
		require.NoError(t, err)

		addWatchable(t, resources, pt)

		//depending on bandwidth, the installation should be finished latest after 30sec
		startTime := time.Now()
		require.NoError(t, pt.Watch(context.TODO(), ReadyState))
		require.WithinDuration(t, startTime, time.Now(), 1*time.Minute)
	})

	t.Run("Test progress tracking to state 'terminated'", func(t *testing.T) {
		cleanup() //delete resources

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		//ensure progress returns error when checking for ready state of terminating resources
		pt1, err := NewProgressTracker(clientSet, logger,
			Config{Interval: 1 * time.Second, Timeout: 2 * time.Second})
		require.NoError(t, err)
		addWatchable(t, resources, pt1)
		require.Error(t, pt1.Watch(ctx, ReadyState)) //error expected as resources could not be watched
		t.Log("Test successfully finished: checking for READY state failed with error")

		//ensure pgoress returns no error when checking for terminated resources
		pt2, err := NewProgressTracker(clientSet, logger,
			Config{Interval: 1 * time.Second, Timeout: 1 * time.Minute})
		require.NoError(t, err)
		addWatchable(t, resources, pt2)

		//Expect NO error as resources are watched until they disappeared
		require.NoError(t, pt2.Watch(ctx, TerminatedState))
		t.Log("Test successfully finished: checking for TERMINATED state finished without an error")
	})
}

func TestDaemonSetRollingUpdate(t *testing.T) {
	test.IntegrationTest(t)

	kubeClient, err := kubeclient.NewKubeClient(test.ReadKubeconfig(t), zap.NewNop().Sugar())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	clientSet, err := kubeClient.GetClientSet()
	require.NoError(t, err)

	testNs := "test-progress-daemonset"
	cleanup := func() {
		t.Log("Cleanup test resources")
		err := clientSet.CoreV1().Namespaces().Delete(ctx, testNs, metav1.DeleteOptions{})
		require.NoError(t, err)
	}
	defer cleanup()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNs}}
	_, err = clientSet.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("Creating daemon set")

	ds := readManifest(t, "ds-before-rolling-update.yaml")[0]
	_, err = kubeClient.ApplyWithNamespaceOverride(ds, testNs)
	require.NoError(t, err)
	time.Sleep(time.Second)

	logger := log.NewLogger(true)
	tracker, err := NewProgressTracker(clientSet, logger, Config{Interval: 1 * time.Second, Timeout: 3 * time.Minute})
	require.NoError(t, err)

	tracker.AddResource(DaemonSet, ds.GetNamespace(), ds.GetName())
	err = tracker.Watch(ctx, ReadyState)
	require.NoError(t, err)

	t.Log("Updating daemon set")

	ds = readManifest(t, "ds-after-rolling-update.yaml")[0]
	_, err = kubeClient.ApplyWithNamespaceOverride(ds, testNs)
	require.NoError(t, err)
	time.Sleep(time.Second)

	tracker, err = NewProgressTracker(clientSet, logger, Config{Interval: 1 * time.Second, Timeout: 3 * time.Minute})
	require.NoError(t, err)

	tracker.AddResource(DaemonSet, ds.GetNamespace(), ds.GetName())
	err = tracker.Watch(ctx, ReadyState)
	require.NoError(t, err)
}

func TestStatefulSetRollingUpdate(t *testing.T) {
	test.IntegrationTest(t)

	kubeClient, err := kubeclient.NewKubeClient(test.ReadKubeconfig(t), zap.NewNop().Sugar())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	clientSet, err := kubeClient.GetClientSet()
	require.NoError(t, err)

	testNs := "test-progress-statefulset"
	cleanup := func() {
		t.Log("Cleanup test resources")
		err := clientSet.CoreV1().Namespaces().Delete(ctx, testNs, metav1.DeleteOptions{})
		require.NoError(t, err)
	}
	defer cleanup()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNs}}
	_, err = clientSet.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("Creating stateful set")

	ss := readManifest(t, "ss-before-rolling-update.yaml")[0]
	_, err = kubeClient.ApplyWithNamespaceOverride(ss, testNs)
	require.NoError(t, err)
	time.Sleep(time.Second)

	logger := log.NewLogger(true)
	tracker, err := NewProgressTracker(clientSet, logger, Config{Interval: 1 * time.Second, Timeout: 3 * time.Minute})
	require.NoError(t, err)

	tracker.AddResource(StatefulSet, ss.GetNamespace(), ss.GetName())
	err = tracker.Watch(ctx, ReadyState)
	require.NoError(t, err)

	t.Log("Updating stateful set")

	ss = readManifest(t, "ss-after-rolling-update.yaml")[0]
	_, err = kubeClient.ApplyWithNamespaceOverride(ss, testNs)
	require.NoError(t, err)
	time.Sleep(time.Second)

	tracker, err = NewProgressTracker(clientSet, logger, Config{Interval: 1 * time.Second, Timeout: 3 * time.Minute})
	require.NoError(t, err)

	tracker.AddResource(StatefulSet, ss.GetNamespace(), ss.GetName())
	err = tracker.Watch(ctx, ReadyState)
	require.NoError(t, err)
}

func TestDeploymentRollingUpdate(t *testing.T) {
	test.IntegrationTest(t)

	kubeClient, err := kubeclient.NewKubeClient(test.ReadKubeconfig(t), zap.NewNop().Sugar())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	clientSet, err := kubeClient.GetClientSet()
	require.NoError(t, err)

	testNs := "test-progress-deployment"
	cleanup := func() {
		t.Log("Cleanup test resources")
		err := clientSet.CoreV1().Namespaces().Delete(ctx, testNs, metav1.DeleteOptions{})
		require.NoError(t, err)
	}
	defer cleanup()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNs}}
	_, err = clientSet.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("Creating deployment")

	dep := readManifest(t, "dep-before-rolling-update.yaml")[0]
	_, err = kubeClient.ApplyWithNamespaceOverride(dep, testNs)
	require.NoError(t, err)
	time.Sleep(time.Second)

	logger := log.NewLogger(true)
	tracker, err := NewProgressTracker(clientSet, logger, Config{Interval: 1 * time.Second, Timeout: 3 * time.Minute})
	require.NoError(t, err)

	tracker.AddResource(Deployment, dep.GetNamespace(), dep.GetName())
	err = tracker.Watch(ctx, ReadyState)
	require.NoError(t, err)

	t.Log("Updating deployment")

	dep = readManifest(t, "dep-after-rolling-update.yaml")[0]
	_, err = kubeClient.ApplyWithNamespaceOverride(dep, testNs)
	require.NoError(t, err)
	time.Sleep(time.Second)

	tracker, err = NewProgressTracker(clientSet, logger, Config{Interval: 1 * time.Second, Timeout: 3 * time.Minute})
	require.NoError(t, err)

	tracker.AddResource(Deployment, dep.GetNamespace(), dep.GetName())
	err = tracker.Watch(ctx, ReadyState)
	require.NoError(t, err)
}

func addWatchable(t *testing.T, resources []*unstructured.Unstructured, pt *Tracker) {
	var cntWatchable int
	for _, resource := range resources {
		watchable, err := NewWatchableResource(resource.GetKind())
		if err == nil {
			t.Logf("Register watchable %s '%s' in namespace '%s'",
				resource.GetKind(), resource.GetName(), resource.GetNamespace())
			pt.AddResource(watchable, resource.GetNamespace(), resource.GetName())
			cntWatchable++
		}
	}
	require.Equal(t, 4, cntWatchable)
}

func readManifest(t *testing.T, filename string) []*unstructured.Unstructured {
	manifest, err := ioutil.ReadFile(filepath.Join("testdata", filename))
	require.NoError(t, err)

	var result []*unstructured.Unstructured
	for _, resourceYAML := range strings.Split(string(manifest), "---") {
		if strings.TrimSpace(resourceYAML) == "" {
			continue
		}
		obj := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, _, err := dec.Decode([]byte(resourceYAML), nil, obj)
		require.NoError(t, err)

		result = append(result, obj)
	}

	return result
}
