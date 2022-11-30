package merge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	clientsetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset/mocks"
	"github.com/kyma-project/istio/operator/api/v1alpha1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ingressgatewayTestUtils "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions/test-utils"
)

const (
	istioManifest = `{"kind":"IstioOperator","apiVersion":"install.istio.io/v1alpha1","metadata":{"name":"default-operator","namespace":"istio-system","creationTimestamp":null},"spec":{"meshConfig":{"accessLogEncoding":"JSON","defaultConfig":{"holdApplicationUntilProxyStarts":true}},"components":{"cni":{"enabled":true}}}}`
)

func createClient(t *testing.T, objects ...client.Object) client.Client {
	err := v1alpha1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objects...).Build()

	return client
}

func Test_IstioOperatorConfiguration(t *testing.T) {
	kubeConfig := "kubeconfig"
	log := logger.NewLogger(false)
	ctx := context.Background()
	defer ctx.Done()

	t.Run("should return error when kubeclient could not be retrieved", func(t *testing.T) {
		// given
		provider := clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.AnythingOfType("string")).Return(nil, errors.New("Istio client error"))
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(k8sClientFake.NewSimpleClientset(), nil)

		// when
		outputManifest, err := IstioOperatorConfiguration(ctx, &provider, istioManifest, kubeConfig, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio client error")
		require.Equal(t, outputManifest, "")
	})

	t.Run("should return default configuration, when there are no Istio CR", func(t *testing.T) {
		// given
		client := createClient(t)
		provider := &clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.AnythingOfType("string")).Return(client, nil)
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(k8sClientFake.NewSimpleClientset(), nil)

		// when
		outputManifest, err := IstioOperatorConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		require.Equal(t, outputManifest, istioManifest)
	})

	t.Run("should return merged configuration, when there is a Istio CR with valid configuration", func(t *testing.T) {
		// given
		iop := istioOperator.IstioOperator{}
		numTrustedProxies := 4
		istioCR := &v1alpha1.Istio{ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-test",
			Namespace: "namespace",
		},
			Spec: v1alpha1.IstioSpec{
				Config: v1alpha1.Config{
					NumTrustedProxies: &numTrustedProxies,
				},
			},
		}

		client := createClient(t, istioCR)
		provider := &clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.AnythingOfType("string")).Return(client, nil)
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(k8sClientFake.NewSimpleClientset(), nil)

		// when
		outputManifest, err := IstioOperatorConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		err = json.Unmarshal([]byte(outputManifest), &iop)
		require.NoError(t, err)
		require.Equal(t, float64(4), iop.Spec.MeshConfig.Fields["defaultConfig"].
			GetStructValue().Fields["gatewayTopology"].GetStructValue().Fields["numTrustedProxies"].GetNumberValue())

	})
}

const TestConfigMap string = `
accessLogEncoding: JSON
defaultConfig:
  discoveryAddress: istiod.istio-system.svc:15012
  gatewayTopology:
    numTrustedProxies: 3
  proxyMetadata: {}
  tracing:
    sampling: 100
    zipkin:
      address: zipkin.kyma-system:9411
enableTracing: true
rootNamespace: istio-system
trustDomain: cluster.local
`

func Test_NeedsIngressGatewayRestart(t *testing.T) {
	kubeConfig := "kubeconfig"
	log := logger.NewLogger(false)
	ctx := context.Background()
	defer ctx.Done()

	t.Run("should return true if CM configuration differs from Istio CR config", func(t *testing.T) {
		//given
		client := ingressgatewayTestUtils.GetIGClient(t, TestConfigMap)
		iop := istioOperator.IstioOperator{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "istio-system"}}
		numTrustedProxies := 4
		istioCR := &v1alpha1.Istio{ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-test",
			Namespace: "namespace",
		},
			Spec: v1alpha1.IstioSpec{
				Config: v1alpha1.Config{
					NumTrustedProxies: &numTrustedProxies,
				},
			},
		}

		err := client.Create(context.TODO(), &iop)
		require.NoError(t, err)

		err = client.Create(context.TODO(), istioCR)
		require.NoError(t, err)

		provider := clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.AnythingOfType("string")).Return(client, nil)
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(k8sClientFake.NewSimpleClientset(), nil)

		//when
		needs, err := NeedsIngressGatewayRestart(ctx, &provider, kubeConfig, log)

		//then
		require.NoError(t, err)
		require.True(t, needs)
	})

	t.Run("should return false if CM configuration doesn't differ from Istio CR config", func(t *testing.T) {
		//given
		client := ingressgatewayTestUtils.GetIGClient(t, TestConfigMap)
		iop := istioOperator.IstioOperator{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "istio-system"}}
		numTrustedProxies := 3
		istioCR := &v1alpha1.Istio{ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-test",
			Namespace: "namespace",
		},
			Spec: v1alpha1.IstioSpec{
				Config: v1alpha1.Config{
					NumTrustedProxies: &numTrustedProxies,
				},
			},
		}

		err := client.Create(context.TODO(), &iop)
		require.NoError(t, err)

		err = client.Create(context.TODO(), istioCR)
		require.NoError(t, err)

		provider := clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.AnythingOfType("string")).Return(client, nil)
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(k8sClientFake.NewSimpleClientset(), nil)

		//when
		needs, err := NeedsIngressGatewayRestart(ctx, &provider, kubeConfig, log)

		//then
		require.NoError(t, err)
		require.False(t, needs)
	})
}
