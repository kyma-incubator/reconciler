package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

const (
	statefulsetInterceptorNS = "unittest-statefulsetinterceptor"
	statefulSetManifestFile  = "statefulsetinterceptor.yaml"
)

func TestStatefulSetInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	kubeClient := newKubeClient(t)
	manifest := readManifest(t, statefulSetManifestFile)

	deleteFct := func() {
		t.Log("Cleanup test resources")
		_, err := kubeClient.Delete(context.TODO(), manifest, statefulsetInterceptorNS)
		require.NoError(t, err)

		//delete finally namespace (will also delete PVC)
		k8sClient, err := kubeClient.Clientset()
		require.NoError(t, err)
		err = k8sClient.CoreV1().Namespaces().Delete(context.TODO(), statefulsetInterceptorNS, v1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			t.Logf("Error occurred during cleanup: %s", err)
		}
	}
	deleteFct()       //Delete before test runs
	defer deleteFct() //Delete after test is finished

	t.Log("Deploying statefulSet")
	deployedResources, err := kubeClient.Deploy(context.TODO(), manifest, statefulsetInterceptorNS, &StatefulSetInterceptor{
		kubeClient: kubeClient,
		logger:     logger.NewLogger(true),
	})
	require.NoError(t, err)
	require.Len(t, deployedResources, 4)

	t.Log("Updating statefulSet")
	updatedResources, err := kubeClient.Deploy(context.TODO(), manifest, statefulsetInterceptorNS, &StatefulSetInterceptor{
		kubeClient: kubeClient,
		logger:     logger.NewLogger(true),
	})
	require.NoError(t, err)
	require.Len(t, updatedResources, 3)
}
