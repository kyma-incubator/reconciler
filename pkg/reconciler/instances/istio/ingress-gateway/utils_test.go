package ingressgateway_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	depNamespace string = "istio-system"
	depName      string = "istio-ingressgateway"
)

func GetClientSet(t *testing.T, configMaps ...string) client.Client {
	deployment := appsv1.Deployment{ObjectMeta: v1.ObjectMeta{Name: depName, Namespace: depNamespace}}

	err := corev1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = appsv1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	if len(configMaps) > 0 {
		data := make(map[string]string)
		data["mesh"] = configMaps[0]
		return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(&corev1.ConfigMap{ObjectMeta: v1.ObjectMeta{Name: "istio", Namespace: "istio-system"}, Data: data}, &deployment).Build()
	}
	return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(&deployment).Build()
}
