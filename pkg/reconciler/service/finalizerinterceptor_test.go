package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestFinalizerInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	manifest, err := os.ReadFile(filepath.Join("test", "finalizerinterceptor.yaml"))
	require.NoError(t, err)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), nil)
	require.NoError(t, err)

	//cleanup
	cleanupFct := func() {
		data := fmt.Sprint(`{"metadata":{"finalizers":[]}}`)
		_ = kubeClient.PatchUsingStrategy(context.Background(), "Namespace", "finalizer-test", "", []byte(data), types.StrategicMergePatchType)
		_, err := kubeClient.Delete(context.Background(), string(manifest), "")
		require.NoError(t, err)
	}
	cleanupFct()       //delete service before test runs
	defer cleanupFct() //delete service after test was finished

	//create LogPipeline in k8s
	_, err = kubeClient.Deploy(context.Background(), string(manifest), "")
	require.NoError(t, err)

	finalizerInterceptor := &FinalizerInterceptor{
		kubeClient:         kubeClient,
		interceptableKinds: []string{"Namespace"},
	}

	namespace := &v1.Namespace{}
	namespace.Name = "finalizer-test"
	unstruct := toUnstructNamespace(t, namespace)
	resList := kubernetes.NewResourceList([]*unstructured.Unstructured{unstruct})
	err = finalizerInterceptor.Intercept(resList, "")
	require.NoError(t, err)

	//validate that finalizer has been added to the intercepted manifest
	require.Contains(t, unstruct.GetFinalizers(), "kubernetes")
}

func toUnstructNamespace(t *testing.T, namespace *v1.Namespace) *unstructured.Unstructured {
	unstructData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(namespace)
	require.NoError(t, err)
	unstructData["kind"] = "Namespace"
	unstruct := &unstructured.Unstructured{Object: unstructData}
	return unstruct
}
