package service

import (
	"context"
	"io/ioutil"
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	hpaInterceptorNS = "unittest-hpainterceptor"
)

func TestHPAInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	manifest, err := ioutil.ReadFile(filepath.Join("test", "hpainterceptor.yaml"))
	require.NoError(t, err)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), &kubernetes.Config{})
	require.NoError(t, err)

	//cleanup
	cleanupFct := func() {
		_, err := kubeClient.Delete(context.Background(), string(manifest), hpaInterceptorNS)
		require.NoError(t, err)
	}
	cleanupFct()       //delete resources before test runs
	defer cleanupFct() //delete resources after test was finished

	hpa := &HPAInterceptor{
		kubeClient: kubeClient,
		logger:     logger.NewLogger(true),
	}

	//deploy test scenario
	_, err = kubeClient.Deploy(context.Background(), string(manifest), hpaInterceptorNS, hpa)
	require.NoError(t, err)
	time.Sleep(2 * time.Second) //give HPA time to react

	//get latest replica from K8s deployment
	deploymentK8s, err := kubeClient.GetDeployment(context.Background(), "deployment", hpaInterceptorNS)
	require.NoError(t, err)
	require.NotEmpty(t, deploymentK8s)
	require.LessOrEqual(t, *deploymentK8s.Spec.Replicas, int32(5)) //5 is max-replica in HPA

	//let interceptor adjust manifest
	unstruct := toUnstruct(t, deploymentK8s)
	resList := kubernetes.NewResourceList([]*unstructured.Unstructured{unstruct})

	err = hpa.Intercept(resList, hpaInterceptorNS)
	require.NoError(t, err)

	deploymentIntrcpt := fromUnstruct(t, resList.Get("Deployment", "deployment", hpaInterceptorNS))
	require.Equal(t, *deploymentK8s.Spec.Replicas, *deploymentIntrcpt.Spec.Replicas)
}

func toUnstruct(t *testing.T, deployment *v1.Deployment) *unstructured.Unstructured {
	unstructData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deployment)
	require.NoError(t, err)
	unstructData["kind"] = "Deployment"
	unstruct := &unstructured.Unstructured{Object: unstructData}
	return unstruct
}

func fromUnstruct(t *testing.T, u *unstructured.Unstructured) *v1.Deployment {
	require.NotEmpty(t, u)
	deploy := &v1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, deploy)
	require.NoError(t, err)
	return deploy
}
