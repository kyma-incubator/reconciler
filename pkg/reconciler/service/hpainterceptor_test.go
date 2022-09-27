package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

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

	manifest, err := os.ReadFile(filepath.Join("test", "hpainterceptor.yaml"))
	require.NoError(t, err)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), nil)
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
	unstruct := toUnstructDeployment(t, deploymentK8s)
	resList := kubernetes.NewResourceList([]*unstructured.Unstructured{unstruct})

	err = hpa.Intercept(resList, hpaInterceptorNS)
	require.NoError(t, err)

	deploymentIntrcpt := fromUnstructDeployment(t, resList.Get("Deployment", "deployment", hpaInterceptorNS))
	require.Equal(t, *deploymentK8s.Spec.Replicas, *deploymentIntrcpt.Spec.Replicas)

	//get latest replica from K8s statefulset
	sfsK8s, err := kubeClient.GetStatefulSet(context.Background(), "sfs", hpaInterceptorNS)
	require.NoError(t, err)
	require.NotEmpty(t, sfsK8s)
	require.LessOrEqual(t, *sfsK8s.Spec.Replicas, int32(5)) //5 is max-replica in HPA

	//let interceptor adjust manifest
	unstruct = toUnstructStatefulset(t, sfsK8s)
	resList = kubernetes.NewResourceList([]*unstructured.Unstructured{unstruct})

	err = hpa.Intercept(resList, hpaInterceptorNS)
	require.NoError(t, err)

	sfsIntrcpt := fromUnstructStatefulset(t, resList.Get("StatefulSet", "sfs", hpaInterceptorNS))
	require.Equal(t, *deploymentK8s.Spec.Replicas, *sfsIntrcpt.Spec.Replicas)
}

func toUnstructDeployment(t *testing.T, deployment *v1.Deployment) *unstructured.Unstructured {
	unstructData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deployment)
	require.NoError(t, err)
	unstructData["kind"] = "Deployment"
	unstruct := &unstructured.Unstructured{Object: unstructData}
	return unstruct
}

func toUnstructStatefulset(t *testing.T, statefulset *v1.StatefulSet) *unstructured.Unstructured {
	unstructData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(statefulset)
	require.NoError(t, err)
	unstructData["kind"] = "StatefulSet"
	unstruct := &unstructured.Unstructured{Object: unstructData}
	return unstruct
}

func fromUnstructDeployment(t *testing.T, u *unstructured.Unstructured) *v1.Deployment {
	require.NotEmpty(t, u)
	deploy := &v1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, deploy)
	require.NoError(t, err)
	return deploy
}

func fromUnstructStatefulset(t *testing.T, u *unstructured.Unstructured) *v1.StatefulSet {
	require.NotEmpty(t, u)
	sfs := &v1.StatefulSet{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sfs)
	require.NoError(t, err)
	return sfs
}
