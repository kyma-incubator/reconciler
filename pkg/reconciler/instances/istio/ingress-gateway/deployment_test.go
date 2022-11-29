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
	newNumTrustedProxies := 2

	istio := istioCR.Istio{}
	istio.Spec.Config.NumTrustedProxies = &newNumTrustedProxies

	does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

	require.NoError(t, err)
	require.True(t, does)

	istio.Spec.Config.NumTrustedProxies = nil
	does, err = ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

	require.NoError(t, err)
	require.True(t, does)

	sameNumTrustedProxies := 3
	istio.Spec.Config.NumTrustedProxies = &sameNumTrustedProxies
	does, err = ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

	require.NoError(t, err)
	require.False(t, does)
}

func TestIngressGatewayNeedsRestartEmptyCM(t *testing.T) {
	client := GetClientSet(t, TestConfigMapEmpty)
	newNumTrustedProxies := 2

	istio := istioCR.Istio{}
	istio.Spec.Config.NumTrustedProxies = &newNumTrustedProxies

	does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

	require.NoError(t, err)
	require.True(t, does)

	istio.Spec.Config.NumTrustedProxies = nil

	does, err = ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{istio}})

	require.NoError(t, err)
	require.False(t, does)
}

func TestIngressGatewayNeedsRestartNoIstioCr(t *testing.T) {
	client := GetClientSet(t, TestConfigMapEmpty)
	does, err := ingressgateway.NeedsRestart(context.TODO(), client, &istioCR.IstioList{Items: []istioCR.Istio{}})

	require.NoError(t, err)
	require.False(t, does)
}

func TestRestartIngressGatewayDeployment(t *testing.T) {
	client := GetClientSet(t, TestConfigMap)

	err := ingressgateway.RestartDeployment(context.TODO(), client)
	require.NoError(t, err)

	dep := appsv1.Deployment{}
	err = client.Get(context.TODO(), types.NamespacedName{Namespace: depNamespace, Name: depName}, &dep)
	require.NoError(t, err)
	require.NotEmpty(t, dep.Spec.Template.Annotations["reconciler.kyma-project.io/lastRestartDate"])
}
