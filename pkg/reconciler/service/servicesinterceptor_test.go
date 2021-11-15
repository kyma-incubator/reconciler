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

func TestServicesInterceptor(t *testing.T) {
	test.IntegrationTest(t)

	manifest, err := ioutil.ReadFile(filepath.Join("test", "service.yaml"))
	require.NoError(t, err)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), &kubernetes.Config{})
	require.NoError(t, err)

	//create service in k8s
	_, err = kubeClient.Deploy(context.Background(), string(manifest), "servicesinterceptor-test")
	require.NoError(t, err)

	svcIntcptr := &ServicesInterceptor{
		kubeClient: kubeClient,
	}

	//get unstruct of service without clusterIP
	unstructs, err := kubernetes.ToUnstructured(manifest, true)
	require.NoError(t, err)
	require.Len(t, unstructs, 1)

	serviceObject, err := toService(unstructs[0])
	require.NoError(t, err)
	require.Empty(t, serviceObject.Spec.ClusterIP)
	t.Logf("ClusterIP before: %s", serviceObject.Spec.ClusterIP)

	//inject clusterIP
	result, err := svcIntcptr.Intercept(unstructs[0])
	require.Equal(t, result, kubernetes.ContinueInterceptionResult)
	require.NoError(t, err)
	serviceObject, err = toService(unstructs[0])
	require.NoError(t, err)
	require.NotEmpty(t, serviceObject.Spec.ClusterIP)
	t.Logf("ClusterIP after: %s", serviceObject.Spec.ClusterIP)

	//update the service in k8s
	manifestIntercepted, err := yaml.Marshal(unstructs[0].Object)
	require.NoError(t, err)
	_, err = kubeClient.Deploy(context.Background(), string(manifestIntercepted), "servicesinterceptor-test")
	require.NoError(t, err)
}

func toService(unstruct *unstructured.Unstructured) (*v1.Service, error) {
	svc := &v1.Service{}
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(unstruct.Object, svc)
	if err != nil {
		return nil, err
	}
	return svc, nil
}
