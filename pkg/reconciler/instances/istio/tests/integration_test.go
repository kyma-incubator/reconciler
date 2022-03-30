package tests

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/avast/retry-go"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/kubectl/pkg/util/podutils"
)

const (
	maxCleanupCallAttempts = 10
	clusterDomainEnvVar    = "CLUSTER_DOMAIN"
	ingressPortEnvVar      = "INGRESS_PORT"
	healthzTestURLFormat   = "http://%s:%s/healthz/ready"
	istioNamespace         = "istio-system"
	pilotIngressGwSelector = "istio in (pilot, ingressgateway)"
	gatewayManifest        = `
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: test-gateway
spec:
  selector:
    istio: ingressgateway # use Istio default gateway implementation
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "*"
`
	virutalServiceManifest = `
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: istio-healthz
spec:
  gateways:
  - istio-system/test-gateway
  hosts:
  - "*"
  http:
  - match:
    - uri:
        exact: /healthz/ready
    route:
    - destination:
        host: istio-ingressgateway.istio-system.svc.cluster.local
        port:
          number: 15021
`
)

var (
	gatewayGVR = schema.GroupVersionResource{Resource: "gateways", Group: "networking.istio.io", Version: "v1beta1"}
	vsGVR      = schema.GroupVersionResource{Resource: "virtualservices", Group: "networking.istio.io", Version: "v1beta1"}
)

func TestIstioIntegration(t *testing.T) {
	skipTestIfDisabled(t)

	setup := newIstioTest(t)
	defer setup.contextCancel()
	clientset, err := setup.kubeClient.Clientset()

	t.Run("istio pods are running and available", func(t *testing.T) {
		options := metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/instance=istio",
		}

		require.NoError(t, err)

		podsList, err := clientset.CoreV1().Pods(istioNamespace).List(setup.context, options)
		require.NoError(t, err)

		for i, pod := range podsList.Items {
			setup.logger.Debugf("Pod %v is deployed", pod.Name)
			require.Equal(t, v1.PodPhase("Running"), pod.Status.Phase)
			require.Equal(t, true, podutils.IsPodAvailable(&podsList.Items[i], 0, metav1.Now()))
		}
	})

	t.Run("istio deployments are synced", func(t *testing.T) {
		pilotSelectorOpt := metav1.ListOptions{LabelSelector: pilotIngressGwSelector}
		deploymentList, err := clientset.AppsV1().Deployments(istioNamespace).List(setup.context, pilotSelectorOpt)
		require.NoError(t, err)
		require.True(t, len(deploymentList.Items) >= 2)
		for _, dep := range deploymentList.Items {
			require.Equal(t, *dep.Spec.Replicas, dep.Status.ReadyReplicas)
			require.Equal(t, *dep.Spec.Replicas, dep.Status.AvailableReplicas)
		}
	})

	t.Run("ingressgw's healthz returns 200 when exposed via Gateway and VirtualService", func(t *testing.T) {
		gateway := setupIstioGateway(t, setup)
		defer cleanupObject(t, setup, gatewayGVR, gateway)
		vs := setupVirtualService(t, setup)
		defer cleanupObject(t, setup, vsGVR, vs)
		healthzURL := readHealthzURL(t)
		var statusCode int
		err := retry.Do(func() error {
			resp, err := http.Get(healthzURL) //nolint:gosec
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			statusCode = resp.StatusCode
			return nil
		}, retry.DelayType(retry.BackOffDelay))
		require.Equal(t, http.StatusOK, statusCode)
		require.NoError(t, err)
	})
}

func readHealthzURL(t *testing.T) string {
	clusterDomain, ok := os.LookupEnv(clusterDomainEnvVar)
	if !ok {
		t.Fatal("CLUSTER_DOMAIN env var is required")
	}
	ingressPort, ok := os.LookupEnv(ingressPortEnvVar)
	if !ok {
		t.Fatal("INGRESS_PORT env var is required")
	}
	healthzURL := fmt.Sprintf(healthzTestURLFormat, clusterDomain, ingressPort)
	return healthzURL
}

func setupIstioGateway(t *testing.T, setup *istioTest) *unstructured.Unstructured {
	gateway := &unstructured.Unstructured{}
	buildObject(t, gateway, gatewayManifest)
	createObject(t, setup, gatewayGVR, gateway)
	return gateway
}

func setupVirtualService(t *testing.T, setup *istioTest) *unstructured.Unstructured {
	vs := &unstructured.Unstructured{}
	buildObject(t, vs, virutalServiceManifest)
	createObject(t, setup, vsGVR, vs)
	return vs
}

func cleanupObject(t *testing.T, setup *istioTest, gvr schema.GroupVersionResource, gw *unstructured.Unstructured) {
	err := retry.Do(func() error {
		deletionPropagation := metav1.DeletePropagationBackground
		err := setup.dynamicClient.Resource(gvr).Namespace(istioNamespace).
			Delete(setup.context, gw.GetName(), metav1.DeleteOptions{PropagationPolicy: &deletionPropagation})
		return err
	}, retry.Attempts(maxCleanupCallAttempts))
	require.NoError(t, err)

}

func createObject(t *testing.T, setup *istioTest, groupVersionResource schema.GroupVersionResource, gateway *unstructured.Unstructured) {
	_, err := setup.dynamicClient.Resource(groupVersionResource).
		Namespace(istioNamespace).
		Create(setup.context, gateway, metav1.CreateOptions{})
	require.NoError(t, err)
}

func buildObject(t *testing.T, vs *unstructured.Unstructured, manifest string) {
	serializer := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err := serializer.Decode([]byte(manifest), nil, vs)
	require.NoError(t, err)
}
