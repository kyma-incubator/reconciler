package ingressgateway_test

import (
	"context"
	"testing"

	ingressgateway "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/ingress-gateway"
	"github.com/stretchr/testify/require"
)

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

const TestConfigMapEmpty string = `
accessLogEncoding: JSON
defaultConfig:
  discoveryAddress: istiod.istio-system.svc:15012
  proxyMetadata: {}
  tracing:
    sampling: 100
    zipkin:
      address: zipkin.kyma-system:9411
enableTracing: true
rootNamespace: istio-system
trustDomain: cluster.local
`

func TestGetNumTrustedProxyFromIstioCM(t *testing.T) {
	t.Run("should return 3 numTrustedProxies when CM has configure 3 numTrustedProxies", func(t *testing.T) {
		client := GetClientSet(t, TestConfigMap)
		numTrustedProxies, err := ingressgateway.GetNumTrustedProxyFromIstioCM(context.TODO(), client)
		require.NoError(t, err)
		require.NotNil(t, numTrustedProxies)
		require.Equal(t, 3, *numTrustedProxies)
	})
}

func TestDefaultNumTrustedProxyFromIstioCM(t *testing.T) {
	t.Run("should return nil when CM has no configuration for numTrustedProxies", func(t *testing.T) {
		client := GetClientSet(t, TestConfigMapEmpty)
		numTrustedProxies, err := ingressgateway.GetNumTrustedProxyFromIstioCM(context.TODO(), client)
		require.NoError(t, err)
		require.Nil(t, numTrustedProxies)
	})
}
