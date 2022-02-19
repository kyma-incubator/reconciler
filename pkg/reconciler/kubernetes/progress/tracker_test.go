package progress

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func TestResourceJSON(t *testing.T) {
	test.IntegrationTest(t)
	// setup
	kubeconfig := test.ReadKubeconfig(t)
	restCfg := test.RestConfig(t, kubeconfig)
	k8s, err := dynamic.NewForConfig(restCfg)
	require.NoError(t, err)
	pt, err := NewProgressTracker(kubernetes.NewForConfigOrDie(restCfg), zap.NewNop().Sugar(), Config{Interval: 1 * time.Second, Timeout: 1 * time.Minute})
	require.NoError(t, err)

	resources := test.ReadManifestToUnstructured(t, "dumpable.yaml")
	require.Len(t, resources, 5)

	cleanup := func() {
		t.Log("Cleanup test resources")
		for _, r := range resources {
			err := k8s.Resource(gvr(r)).Namespace(r.GetNamespace()).Delete(context.Background(), r.GetName(), v1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				t.Fatalf("Failed to delete resource: %s", err)
			}
		}
	}
	cleanup()       //ensure relicts from previous test runs were removed
	defer cleanup() //cleanup after test is finished

	// each resource is created and immediately tested, to avoid separate loops or extra work if the test fails early
	for _, r := range resources {
		_, err := k8s.Resource(gvr(r)).Namespace(r.GetNamespace()).Create(context.Background(), r, v1.CreateOptions{})
		require.NoError(t, err)

		w, err := NewWatchableResource(r.GetKind())
		if err != nil {
			continue // resource is not watchable
		}

		pt.AddResource(w, r.GetNamespace(), r.GetName())

		actual, err := pt.resourceJSON(context.Background(), pt.objects[len(pt.objects)-1])
		require.NoError(t, err)
		require.Contains(t, string(actual), r.GetName())
		require.Contains(t, string(actual), r.GetNamespace())
	}
}

func gvr(r *unstructured.Unstructured) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.GroupVersionKind().Group,
		Version:  r.GroupVersionKind().Version,
		Resource: fmt.Sprintf("%s%s", strings.ToLower(r.GroupVersionKind().Kind), "s"), // usually we use a mapper to find the plural, but it is too much overhead for a test
	}
}
