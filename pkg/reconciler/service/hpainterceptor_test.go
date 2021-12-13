package service

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

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
	cleanupFct()       //delete pvc before test runs
	defer cleanupFct() //delete pvc after test was finished

}
