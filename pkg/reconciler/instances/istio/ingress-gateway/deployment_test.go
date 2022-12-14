package ingressgateway_test

import (
	"context"
	"testing"

	ingressgateway "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/ingress-gateway"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	istioCR "github.com/kyma-project/istio/operator/api/v1alpha1"
)

func TestIngressGatewayNeedsRestart(t *testing.T) {
	client := GetClientSet(t, TestConfigMap)
	istio := istioCR.Istio{}

	t.Run("should restart when CR numTrustedProxies is 2 and CM has configuration for 3", func(t *testing.T) {
		newNumTrustedProxies := 2

		istio.Spec.Config.NumTrustedProxies = &newNumTrustedProxies

		does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

		require.NoError(t, err)
		require.True(t, does)
	})

	t.Run("should restart when CR numTrustedProxies is nil and CM has configuration for numTrustedProxies:3", func(t *testing.T) {
		istio.Spec.Config.NumTrustedProxies = nil
		does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

		require.NoError(t, err)
		require.True(t, does)
	})

	t.Run("should not restart when CR numTrustedProxies is 3 and CM has configuration for numTrustedProxies:3", func(t *testing.T) {
		sameNumTrustedProxies := 3
		istio.Spec.Config.NumTrustedProxies = &sameNumTrustedProxies
		does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

		require.NoError(t, err)
		require.False(t, does)
	})
}

func TestIngressGatewayNeedsRestartEmptyCM(t *testing.T) {
	client := GetClientSet(t, TestConfigMapEmpty)
	istio := istioCR.Istio{}

	t.Run("should restart when CR has numTrustedProxy configured, and CM doesn't have configuration for numTrustedProxies", func(t *testing.T) {
		newNumTrustedProxies := 2
		istio.Spec.Config.NumTrustedProxies = &newNumTrustedProxies

		does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

		require.NoError(t, err)
		require.True(t, does)
	})

	t.Run("should not restart when CR doesn't configure numTrustedProxies, and CM doesn't have configuration for numTrustedProxies", func(t *testing.T) {
		istio.Spec.Config.NumTrustedProxies = nil

		does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

		require.NoError(t, err)
		require.False(t, does)
	})
}

func TestIngressGatewayNeedsRestartNoIstioCr(t *testing.T) {
	t.Run("should not restart when there's no Istio CR, and CM has no configuration for numTrustedProxies", func(t *testing.T) {
		client := GetClientSet(t, TestConfigMapEmpty)
		does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{}})

		require.NoError(t, err)
		require.False(t, does)
	})

	t.Run("should not restart when IstioCRList is nil, and CM has no configuration for numTrustedProxies", func(t *testing.T) {
		client := GetClientSet(t, TestConfigMap)
		does, err := ingressgateway.NeedsRestart(context.TODO(), client, nil)

		require.NoError(t, err)
		require.True(t, does)
	})

	t.Run("should restart when there's no Istio CR, and CM has configuration for numTrustedProxies", func(t *testing.T) {
		client := GetClientSet(t, TestConfigMap)
		does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{}})

		require.NoError(t, err)
		require.True(t, does)
	})

	t.Run("should restart when IstioCRList is nil, and CM has configuration for numTrustedProxies", func(t *testing.T) {
		client := GetClientSet(t, TestConfigMap)
		does, err := ingressgateway.NeedsRestart(context.TODO(), client, nil)

		require.NoError(t, err)
		require.True(t, does)
	})
}

func TestRestartIngressGatewayDeployment(t *testing.T) {
	t.Run("should set annotation on Istio IG deployment when restart is needed", func(t *testing.T) {
		client := GetClientSet(t, TestConfigMap)

		err := ingressgateway.RestartDeployment(context.TODO(), client)
		require.NoError(t, err)

		dep := appsv1.Deployment{}
		err = client.Get(context.TODO(), types.NamespacedName{Namespace: depNamespace, Name: depName}, &dep)
		require.NoError(t, err)
		require.NotEmpty(t, dep.Spec.Template.Annotations["reconciler.kyma-project.io/lastRestartDate"])
	})
}

func TestIngressGatewayNeedsRestartNoCM(t *testing.T) {
	t.Run("should restart when there's no CM", func(t *testing.T) {
		client := GetClientSet(t)
		newNumTrustedProxies := 2

		istio := istioCR.Istio{}
		istio.Spec.Config.NumTrustedProxies = &newNumTrustedProxies

		does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

		require.NoError(t, err)
		require.True(t, does)
	})
}
