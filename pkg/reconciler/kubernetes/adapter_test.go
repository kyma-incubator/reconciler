package kubernetes

import (
	"context"
	"fmt"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"path/filepath"
	"testing"
	"time"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

var expectedResourcesWithoutNs = []*Resource{
	{
		Kind:      "Deployment",
		Name:      "unittest-deployment",
		Namespace: "default",
	},
}

var expectedResourcesWithNs = []*Resource{
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

var expectedLabels = map[string]string{"test-interceptor": "test-label"}

type testInterceptor struct {
	result InterceptionResult
	err    error
}

func (i *testInterceptor) Intercept(resource *unstructured.Unstructured, _ string) (InterceptionResult, error) {
	resource.SetLabels(expectedLabels)
	return i.result, i.err
}

func TestKubernetesClient(t *testing.T) {
	test.IntegrationTest(t)

	//create client
	kubeClient, err := NewKubernetesClient(test.ReadKubeconfig(t), log.NewLogger(true), &Config{
		ProgressInterval: 1 * time.Second,
		ProgressTimeout:  1 * time.Minute,
	})
	require.NoError(t, err)

	t.Run("Deploy no resources because all were sorted out by interceptor", func(t *testing.T) {
		manifestWithNs := readManifest(t, "unittest-with-namespace.yaml")

		//deploy
		deployedResources, err := kubeClient.Deploy(context.TODO(), manifestWithNs, "unittest-adapter", &testInterceptor{
			result: IgnoreResourceInterceptionResult,
		})
		require.NoError(t, err)
		require.Empty(t, deployedResources)
	})

	t.Run("Deploy no resources because interceptor was failing", func(t *testing.T) {
		manifestWithNs := readManifest(t, "unittest-with-namespace.yaml")

		//deploy
		deployedResources, err := kubeClient.Deploy(context.TODO(), manifestWithNs, "unittest-adapter", &testInterceptor{
			result: ErrorInterceptionResult,
			err:    fmt.Errorf("just a fake error"),
		})
		require.Error(t, err)
		require.Empty(t, deployedResources)
	})

	t.Run("Deploy and delete resources with namespace", func(t *testing.T) {
		manifestWithNs := readManifest(t, "unittest-with-namespace.yaml")

		//deploy
		t.Log("Deploying test resources")
		deployedResources, err := kubeClient.Deploy(context.TODO(), manifestWithNs, "unittest-adapter", &testInterceptor{})
		require.NoError(t, err)
		require.ElementsMatch(t, expectedResourcesWithNs, deployedResources)

		//check execution of interceptors
		clientSet, err := kubeClient.Clientset()
		require.NoError(t, err)
		ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), "unittest-adapter", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotEmpty(t, ns.GetLabels()["test-interceptor"])
		require.Equal(t, expectedLabels["test-interceptor"], ns.GetLabels()["test-interceptor"])

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
