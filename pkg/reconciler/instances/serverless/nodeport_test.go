package serverless

import (
	"context"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"testing"
)

const nonConflictPort = 32238

type assertFn func(t *testing.T, overrides map[string]interface{}, occupiedPorts map[int32]struct{})

func TestNodePortAction(t *testing.T) {
	testCases := map[string]struct {
		givenService *corev1.Service
		assertFn     assertFn
	}{
		"Don't set override when nodePort installed on default port": {
			givenService: fixtureServiceNodePort(dockerRegistryService, "", dockerRegistryNodePort),
			assertFn:     assertNoOverride(),
		},
		"Set override when nodePort service installed on different port": {
			givenService: fixtureServiceNodePort(dockerRegistryService, "", nonConflictPort),
			assertFn:     assertOverride(nonConflictPort),
		},
		"Don't set override when nodePort not installed, without port conflict": {
			assertFn: assertNoOverride(),
		},
		"Set override when nodePort not installed, with port conflict": {
			givenService: fixtureServiceNodePort("conflicting-svc", "", dockerRegistryNodePort),
			assertFn:     assertGeneratedPortOverride(),
		},
		"Don't set override when service is ClusterIP before upgrade without port conflict": {
			givenService: fixtureServiceClusterIP(dockerRegistryService),
			assertFn:     assertNoOverride(),
		},
		"Set override when cluster has NodePort service in different namespace with port conflict": {
			givenService: fixtureServiceNodePort(dockerRegistryService, "different-ns", dockerRegistryNodePort),
			assertFn:     assertNoOverride(),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			//GIVEN
			occupiedPorts := map[int32]struct{}{}

			k8sClient, actionContext := setup()
			action := ResolveDockerRegistryNodePort{fixedNodePort(nonConflictPort)}
			if testCase.givenService != nil {
				_, err := k8sClient.CoreV1().Services(actionContext.Task.Namespace).Create(context.TODO(), testCase.givenService, metav1.CreateOptions{})
				require.NoError(t, err)
				occupiedPorts[testCase.givenService.Spec.Ports[0].NodePort] = struct{}{}
			}
			fixtureServices(t, k8sClient, actionContext.Task.Namespace, occupiedPorts)

			//WHEN
			err := action.Run(actionContext)

			//THEN
			require.NoError(t, err)
			require.NotNil(t, testCase.assertFn)
			testCase.assertFn(t, actionContext.Task.Configuration, occupiedPorts)
		})
	}
}

func assertNoOverride() assertFn {
	return func(t *testing.T, overrides map[string]interface{}, occupiedPorts map[int32]struct{}) {
		_, ok := overrides[dockerRegistryNodePortPath]
		require.False(t, ok)
	}
}

func assertGeneratedPortOverride() assertFn {
	return func(t *testing.T, overrides map[string]interface{}, occupiedPorts map[int32]struct{}) {
		value, ok := overrides[dockerRegistryNodePortPath]
		require.True(t, ok)
		out, ok := value.(int32)
		require.True(t, ok)
		require.NotContains(t, occupiedPorts, out)
	}
}

func assertOverride(port int32) assertFn {
	return func(t *testing.T, overrides map[string]interface{}, occupiedPorts map[int32]struct{}) {
		value, ok := overrides[dockerRegistryNodePortPath]
		require.True(t, ok)
		out, ok := value.(int32)
		require.True(t, ok)
		require.Equal(t, port, out)
	}
}

func fixtureServiceNodePort(name, namespace string, nodePort int32) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{Name: dockerRegistryPortName, NodePort: nodePort}},
		},
	}
}

func fixtureServiceClusterIP(name string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{Name: dockerRegistryPortName, Port: 5000}},
		},
	}
}

func fixtureServices(T *testing.T, k8sClient kubernetes.Interface, namespace string, occupiedPorts map[int32]struct{}) {
	svc := fixtureServiceNodePort("other-node-port", "", dockerRegistryNodePort-1)
	_, err := k8sClient.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(T, err)
	occupiedPorts[dockerRegistryNodePort-1] = struct{}{}

	svc = fixtureServiceNodePort("many-ports", "", dockerRegistryNodePort+2)
	occupiedPorts[dockerRegistryNodePort+2] = struct{}{}
	svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
		Name:     "test",
		NodePort: 33333,
	})

	_, err = k8sClient.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(T, err)
	occupiedPorts[33333] = struct{}{}
}

func fixedNodePort(expectedPort int32) func() int32 {
	return func() int32 {
		return expectedPort
	}
}
