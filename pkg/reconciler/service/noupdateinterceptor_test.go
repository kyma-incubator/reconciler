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
	noUpdateInterceptorNS = "unittest-noupdateinterceptor"
)

func TestNoUpdateInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	kubeClient := newKubeClient(t)
	manifestData := readManifest(t)

	deleteFct := func() {
		t.Log("Cleanup test resources")
		_, err := kubeClient.Delete(context.TODO(), string(manifestData), noUpdateInterceptorNS)
		require.NoError(t, err)
	}
	deleteFct()       //Delete before test runs
	defer deleteFct() //Delete after test is finished

	t.Log("Deploying resources")
	deployedResources, err := kubeClient.Deploy(context.TODO(), string(manifestData), noUpdateInterceptorNS, &NoUpdateInterceptor{
		kubeClient: kubeClient,
		logger:     logger.NewLogger(true),
	})
	require.NoError(t, err)
	require.Len(t, deployedResources, 3)

	t.Log("Updating resources")
	updatedResources, err := kubeClient.Deploy(context.TODO(), string(manifestData), noUpdateInterceptorNS, &NoUpdateInterceptor{
		kubeClient: kubeClient,
		logger:     logger.NewLogger(true),
	})
	require.NoError(t, err)
	require.Len(t, updatedResources, 1)
}

func readManifest(t *testing.T) []byte {
	manifestData, err := ioutil.ReadFile(filepath.Join("test", "noupdateinterceptor.yaml"))
	require.NoError(t, err)
	return manifestData
}

func newKubeClient(t *testing.T) kubernetes.Client {
	//create client
	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), &kubernetes.Config{
		ProgressInterval: 1 * time.Second,
		ProgressTimeout:  1 * time.Minute,
	})
	require.NoError(t, err)
	return kubeClient
}
