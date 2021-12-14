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
	pvcInterceptorNS = "unittest-pvcinterceptor"
)

func TestPVCInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	manifest, err := ioutil.ReadFile(filepath.Join("test", "pvcinterceptor.yaml"))
	require.NoError(t, err)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), &kubernetes.Config{})
	require.NoError(t, err)

	//cleanup
	cleanupFct := func() {
		_, err := kubeClient.Delete(context.Background(), string(manifest), pvcInterceptorNS)
		require.NoError(t, err)
	}
	cleanupFct()       //delete pvc before test runs
	defer cleanupFct() //delete pvc after test was finished

	//create pvc in k8s multiple times: no error expected
	applyPVC(t, kubeClient, manifest)
}

func applyPVC(t *testing.T, kubeClient kubernetes.Client, manifest []byte) {
	for i := 0; i < 3; i++ {
		_, err := kubeClient.Deploy(context.Background(), string(manifest), pvcInterceptorNS, &PVCInterceptor{
			kubeClient: kubeClient,
			logger:     logger.NewLogger(true),
		})
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	}
}
