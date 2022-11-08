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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	istioManifest = `{"kind":"IstioOperator","apiVersion":"install.istio.io/v1alpha1","metadata":{"name":"default-operator","namespace":"istio-system","creationTimestamp":null},"spec":{"meshConfig":{"accessLogEncoding":"JSON","defaultConfig":{"holdApplicationUntilProxyStarts":true}}}}`
)

func createClient(t *testing.T, objects ...client.Object) client.Client {
	err := v1alpha1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)
	err = corev1.AddToScheme(scheme.Scheme)
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

		// when
		manifest, err := IstioOperatorConfiguration(ctx, &provider, istioManifest, kubeConfig, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio client error")
		require.Equal(t, manifest, "")
	})

	t.Run("should return default configuration, when there are no Istio CR", func(t *testing.T) {
		// given
		provider := &clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.AnythingOfType("string")).Return(createClient(t), nil)

		// when
		manifest, err := IstioOperatorConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		require.Equal(t, manifest, istioManifest)
	})

	t.Run("should return merged configuration, when there is a Istio CR with valid configuration", func(t *testing.T) {
		// given
		iop := istioOperator.IstioOperator{}
		numTrustedProxies := 4
		istioCR := &v1alpha1.Istio{ObjectMeta: v1.ObjectMeta{
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

		// when
		manifest, err := IstioOperatorConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		json.Unmarshal([]byte(manifest), &iop)
		require.Equal(t, float64(4), iop.Spec.MeshConfig.Fields["defaultConfig"].
			GetStructValue().Fields["gatewayTopology"].GetStructValue().Fields["numTrustedProxies"].GetNumberValue())

	})
}
