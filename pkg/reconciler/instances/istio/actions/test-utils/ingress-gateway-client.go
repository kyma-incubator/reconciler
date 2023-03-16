package testutils

import (
	"testing"

	"github.com/kyma-project/istio/operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/require"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	controllerfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	DepNamespace string = "istio-system"
	DepName      string = "istio-ingressgateway"
)

func GetIGClient(t *testing.T, configMaps ...string) client.Client {
	deployment := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: DepName, Namespace: DepNamespace}}

	err := corev1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = appsv1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = v1alpha1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = istioOperator.SchemeBuilder.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	if len(configMaps) > 0 {
		data := make(map[string]string)
		data["mesh"] = configMaps[0]
		return controllerfake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "istio", Namespace: "istio-system"}, Data: data}, &deployment).Build()
	}

	return controllerfake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(&deployment).Build()
}
