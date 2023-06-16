package serverless

import (
	"context"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"testing"
)

const nonConflictPort int32 = 32238

const KymaNamespace = "kyma-system"

type assertFn func(t *testing.T, overrides map[string]interface{})

func TestNodePortAction(t *testing.T) {
	testCases := map[string]struct {
		givenService *corev1.Service
		assertFn     assertFn
	}{
		"Don't set override when nodePort installed on default port": {
			givenService: fixtureServiceNodePort(dockerRegistryService, KymaNamespace, dockerRegistryNodePort),
			assertFn:     assertNoOverride(),
		},
		"Set override when nodePort service installed on different port": {
			givenService: fixtureServiceNodePort(dockerRegistryService, KymaNamespace, nonConflictPort),
			assertFn:     assertGeneratedPortOverride(),
		},
		"Don't set override when nodePort not installed, without port conflict": {
			assertFn: assertNoOverride(),
		},
		"Set override when nodePort not installed, with port conflict": {
			givenService: fixtureServiceNodePort("conflicting-svc", KymaNamespace, dockerRegistryNodePort),
			assertFn:     assertGeneratedPortOverride(),
		},
		"Don't set override when service is ClusterIP before upgrade without port conflict": {
			givenService: fixtureServiceClusterIP(dockerRegistryService, KymaNamespace),
			assertFn:     assertNoOverride(),
		},
		"Set override when cluster has NodePort service in different namespace with port conflict": {
			givenService: fixtureServiceNodePort(dockerRegistryService, "different-ns", dockerRegistryNodePort),
			assertFn:     assertGeneratedPortOverride(),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			//GIVEN

			k8sClient, actionContext := setup()
			actionContext.Task.Namespace = KymaNamespace
			action := ResolveDockerRegistryNodePort{fixedNodePort(nonConflictPort)}
			if testCase.givenService != nil {
				_, err := k8sClient.CoreV1().Services(testCase.givenService.Namespace).Create(context.TODO(), testCase.givenService, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			fixtureServices(t, k8sClient, actionContext.Task.Namespace)

			//WHEN
			err := action.Run(actionContext)

			//THEN
			require.NoError(t, err)
			require.NotNil(t, testCase.assertFn)
			testCase.assertFn(t, actionContext.Task.Configuration)
		})
	}
}

func assertNoOverride() assertFn {
	return func(t *testing.T, overrides map[string]interface{}) {
		_, ok := overrides[dockerRegistryNodePortPath]
		require.False(t, ok)
	}
}

func assertGeneratedPortOverride() assertFn {
	return func(t *testing.T, overrides map[string]interface{}) {
		value, ok := overrides[dockerRegistryNodePortPath]
		require.True(t, ok)
		override, ok := value.(int32)
		require.True(t, ok)
		require.Equal(t, nonConflictPort, override)
	}
}

func assertOverride(port int32) assertFn {
	return func(t *testing.T, overrides map[string]interface{}) {
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

func fixtureServiceClusterIP(name, namespace string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIPs
			git,
			Ports: []corev1.ServicePort{
				{Name: dockerRegistryPortName, Port: 5000}},
		},
	}
}

func fixtureServices(T *testing.T, k8sClient kubernetes.Interface, namespace string) {
	svc := fixtureServiceNodePort("other-node-port", KymaNamespace, dockerRegistryNodePort-1)
	_, err := k8sClient.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(T, err)

	svc = fixtureServiceNodePort("many-ports", KymaNamespace, dockerRegistryNodePort+2)
	svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
		Name:     "test",
		NodePort: 33333,
	})

	_, err = k8sClient.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	require.NoError(T, err)
}

func fixedNodePort(expectedPort int32) func() int32 {
	return func() int32 {
		return expectedPort
	}
}
