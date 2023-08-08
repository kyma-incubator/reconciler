package service

import (
	"context"
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
)

const (
	pvcInterceptorNS = "unittest-pvcinterceptor-pvc"
)

func TestPVCInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), nil)
	require.NoError(t, err)

	t.Run("Test Volume Expanded PersistentVolumeClaim interception", func(t *testing.T) {
		manifest, err := os.ReadFile(filepath.Join("test", "pvcinterceptor-pvc.yaml"))
		require.NoError(t, err)

		pvcInterceptor := &PVCInterceptor{
			kubeClient: kubeClient,
			logger:     logger.NewLogger(true),
		}

		//cleanup
		cleanupFct := func() {
			_, err := kubeClient.Delete(context.Background(), string(manifest), pvcInterceptorNS)
			require.NoError(t, err)
		}
		cleanupFct()       //delete pvc before test runs
		defer cleanupFct() //delete pvc after test was finished

		//deploy a pvc with 2Mi Volume - assume it was expanded by user (and kubernetes allowed that due to binding and storageClass conditions)
		_, err = kubeClient.Deploy(context.Background(), string(manifest), pvcInterceptorNS, pvcInterceptor)
		require.NoError(t, err)

		//get pvc from K8s <- original PVC on the runtime
		pvcK8s, err := kubeClient.GetPersistentVolumeClaim(context.Background(), "pvc", pvcInterceptorNS)
		require.NoError(t, err)
		require.NotEmpty(t, pvcK8s)
		require.Equal(t, "2Mi", pvcK8s.Spec.Resources.Requests.Storage().String())

		//fix desired target PVC unstruct with volume size equal 1Mi
		targetPVCUnstruct := toUnstructPVC(t, pvcK8s)
		err = unstructured.SetNestedField(targetPVCUnstruct.Object, "1Mi", "spec", "resources", "requests", "storage")
		require.NoError(t, err)
		targetPvc := fromUnstructPVC(t, targetPVCUnstruct)
		require.Equal(t, "1Mi", targetPvc.Spec.Resources.Requests.Storage().String())

		//let the interceptor adjust the 1Mi to 2Mi
		resList := kubernetes.NewResourceList([]*unstructured.Unstructured{targetPVCUnstruct})
		err = pvcInterceptor.Intercept(resList, pvcInterceptorNS)
		require.NoError(t, err)

		//verify value adjustment
		pvcIntrcepted := fromUnstructPVC(t, resList.Get("PersistentVolumeClaim", "pvc", pvcInterceptorNS))
		require.Equal(t, "2Mi", pvcIntrcepted.Spec.Resources.Requests.Storage().String())
	})
}

func fromUnstructPVC(t *testing.T, u *unstructured.Unstructured) *v1.PersistentVolumeClaim {
	require.NotEmpty(t, u)
	pvc := &v1.PersistentVolumeClaim{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pvc)
	require.NoError(t, err)
	return pvc
}

func toUnstructPVC(t *testing.T, pvc *v1.PersistentVolumeClaim) *unstructured.Unstructured {
	unstructData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pvc)
	require.NoError(t, err)
	unstructData["kind"] = "PersistentVolumeClaim"
	unstruct := &unstructured.Unstructured{Object: unstructData}
	return unstruct
}
