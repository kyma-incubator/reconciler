package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"
)

const (
	pvcInterceptorNS = "unittest-pvcinterceptor-pvc"
	sfsInterceptorNS = "unittest-pvcinterceptor-sfs"
)

func TestPVCInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), &kubernetes.Config{})
	require.NoError(t, err)

	t.Run("Test PersistentVolumeClaim interception", func(t *testing.T) {
		manifest, err := ioutil.ReadFile(filepath.Join("test", "pvcinterceptor-pvc.yaml"))
		require.NoError(t, err)

		//cleanup
		cleanupFct := func() {
			_, err := kubeClient.Delete(context.Background(), string(manifest), pvcInterceptorNS)
			require.NoError(t, err)
		}
		cleanupFct()       //delete pvc before test runs
		defer cleanupFct() //delete pvc after test was finished

		//create pvc in k8s multiple times: no error expected
		for i := 0; i < 3; i++ {
			_, err := kubeClient.Deploy(context.Background(), string(manifest), pvcInterceptorNS, &PVCInterceptor{
				kubeClient: kubeClient,
				logger:     logger.NewLogger(true),
			})
			require.NoError(t, err)
			time.Sleep(100 * time.Millisecond)
		}
	})

	t.Run("Test StatefulSet interception", func(t *testing.T) {
		manifest, err := ioutil.ReadFile(filepath.Join("test", "pvcinterceptor-sfs.yaml"))
		require.NoError(t, err)

		kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), nil)
		require.NoError(t, err)

		//cleanup
		cleanupFct := func() {
			_, err := kubeClient.Delete(context.Background(), string(manifest), sfsInterceptorNS)
			require.NoError(t, err)
		}
		cleanupFct()       //delete sfs before test runs
		defer cleanupFct() //delete sfs after test was finished

		//create pvc in k8s multiple times: no error expected
		for i := 0; i < 3; i++ {
			_, err := kubeClient.Deploy(context.Background(), string(manifest), sfsInterceptorNS, &PVCInterceptor{
				kubeClient: kubeClient,
				logger:     logger.NewLogger(true),
			})
			require.NoError(t, err)
			time.Sleep(100 * time.Millisecond)
		}
	})

}
