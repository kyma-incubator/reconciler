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
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	servicesInterceptorNS = "unittest-servicesinterceptor"
)

func TestServicesInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	manifest, err := ioutil.ReadFile(filepath.Join("test", "servicesinterceptor.yaml"))
	require.NoError(t, err)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), &kubernetes.Config{})
	require.NoError(t, err)

	//cleanup
	cleanupFct := func() {
		_, err := kubeClient.Delete(context.Background(), string(manifest), servicesInterceptorNS)
		require.NoError(t, err)
	}
	cleanupFct()       //delete service before test runs
	defer cleanupFct() //delete service after test was finished

	//create service in k8s
	_, err = kubeClient.Deploy(context.Background(), string(manifest), servicesInterceptorNS)
	require.NoError(t, err)

	svcIntcptr := &ServicesInterceptor{
		kubeClient: kubeClient,
	}

	//get unstruct of service without clusterIP
	unstructs, err := kubernetes.ToUnstructured(manifest, true)
	require.NoError(t, err)
	require.Len(t, unstructs, 2)

	testAssertions := func(t *testing.T, service *unstructured.Unstructured, expectedClusterIP string) {
		serviceObject := toService(t, service)
		require.Equal(t, serviceObject.Spec.ClusterIP, expectedClusterIP)
		t.Logf("ClusterIP before: %s", serviceObject.Spec.ClusterIP)

		//inject clusterIP
		resources := kubernetes.NewResourceList([]*unstructured.Unstructured{service})
		err = svcIntcptr.Intercept(resources, servicesInterceptorNS)
		require.NoError(t, err)

		serviceObject = toService(t, service)
		require.NotEmpty(t, serviceObject.Spec.ClusterIP)
		t.Logf("ClusterIP after: %s", serviceObject.Spec.ClusterIP)

		//update the service in k8s
		manifestIntercepted, err := yaml.Marshal(service.Object)
		require.NoError(t, err)
		_, err = kubeClient.Deploy(context.Background(), string(manifestIntercepted), servicesInterceptorNS)
		require.NoError(t, err)
	}

	// check with empty clusterIP
	service := unstructs[0]
	testAssertions(t, service, "")

	// check with "None" clusterIP
	service = unstructs[1]
	testAssertions(t, service, none)
}

func toService(t *testing.T, unstruct *unstructured.Unstructured) *v1.Service {
	svc := &v1.Service{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, svc)
	require.NotEmpty(t, err)
	return svc
}
