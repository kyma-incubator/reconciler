package merge

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	clientsetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset/mocks"
	"github.com/kyma-project/istio/operator/api/v1alpha1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	istioManifest = `{"kind":"IstioOperator","apiVersion":"install.istio.io/v1alpha1","metadata":{"name":"default-operator","namespace":"istio-system","creationTimestamp":null},"spec":{"meshConfig":{"accessLogEncoding":"JSON"}}}`
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

	t.Run("should return default configuration", func(t *testing.T) {
		// given
		provider := &clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.AnythingOfType("string")).Return(createClient(t), nil)

		// when
		manifest, err := IstioOperatorConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		require.Equal(t, manifest, istioManifest)
	})
}
