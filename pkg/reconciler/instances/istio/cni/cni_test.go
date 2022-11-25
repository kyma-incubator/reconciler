package cni

/* func Test_isCNIEnabled(t *testing.T) {
	t.Run("should get the CNI enabled value from istio chart", func(t *testing.T) {
		// given
		branch := "branch"
		istioChart := "istio-cni-enabled"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)

		// when
		cniEnabled, err := isCniEnabledInChart(factory, branch, istioChart)

		// then
		require.NoError(t, err)
		require.EqualValues(t, true, cniEnabled)
	})
}

func Test_isCNIRolloutRequired(t *testing.T) {
	branch := "branch"
	istioChart := "istio-cni-enabled"
	log := logger.NewLogger(false)
	t.Run("should start proxy reset if CNI config map change the default Helm chart value", func(t *testing.T) {
		// given
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)
		configMapValueString := "false"
		istioCNIConfigMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      configMapCNI,
			Namespace: kymaNamespace,
		},
			Data: map[string]string{"enabled": configMapValueString},
		}
		client := fake.NewSimpleClientset(istioCNIConfigMap)

		// when
		proxyRolloutRequired, err := IsRolloutRequired(context.TODO(), client, factory, branch, istioChart, log)

		// then
		require.NoError(t, err)
		require.EqualValues(t, true, proxyRolloutRequired)
	})
	t.Run("should not start proxy reset if CNI config map does not change the default Helm chart value", func(t *testing.T) {
		// given
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)
		configMapValueString := "true"
		istioCNIConfigMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      configMapCNI,
			Namespace: kymaNamespace,
		},
			Data: map[string]string{"enabled": configMapValueString},
		}
		client := fake.NewSimpleClientset(istioCNIConfigMap)

		// when
		proxyRolloutRequired, err := IsRolloutRequired(context.TODO(), client, factory, branch, istioChart, log)

		// then
		require.NoError(t, err)
		require.EqualValues(t, false, proxyRolloutRequired)
	})
}
*/

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	clientsetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"
	operatorv1alpha1 "istio.io/api/operator/v1alpha1"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
)

const (
	istioManifest = `{"kind":"IstioOperator","apiVersion":"install.istio.io/v1alpha1","metadata":{"name":"default-operator","namespace":"istio-system","creationTimestamp":null},"spec":{"meshConfig":{"accessLogEncoding":"JSON","defaultConfig":{"holdApplicationUntilProxyStarts":true}},"components":{"cni":{"enabled":true}}}}`
)

func createClient(t *testing.T, object runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	err := istioOperator.SchemeBuilder.AddToScheme(scheme)
	require.NoError(t, err)
	client := dynamicfake.NewSimpleDynamicClient(scheme, object)

	return client
}

func Test_ApplyCNIConfiguration(t *testing.T) {
	kubeConfig := "kubeconfig"
	log := logger.NewLogger(false)
	ctx := context.Background()
	defer ctx.Done()

	t.Run("should return error when kubeclient could not be retrieved", func(t *testing.T) {
		// given
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil, errors.New("Istio client error"))

		// when
		outputManifest, err := ApplyCNIConfiguration(ctx, &provider, istioManifest, kubeConfig, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio client error")
		require.Equal(t, outputManifest, "")
	})
	t.Run("should return merged configuration, when there is a Istio CM with different configuration", func(t *testing.T) {
		// given
		configMapValueString := "false"
		configMapValueBool, err := strconv.ParseBool(configMapValueString)
		require.NoError(t, err)
		iop := istioOperator.IstioOperator{}
		istioCNIConfigMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      configMapCNI,
			Namespace: kymaNamespace,
		},
			Data: map[string]string{"enabled": configMapValueString},
		}
		client := k8sClientFake.NewSimpleClientset(istioCNIConfigMap)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)

		// when
		outputManifest, err := ApplyCNIConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		err = json.Unmarshal([]byte(outputManifest), &iop)
		require.NoError(t, err)
		require.Equal(t, configMapValueBool, iop.Spec.Components.Cni.Enabled.GetValue())

	})
	t.Run("should not return merged configuration, when there is a Istio CM with invalid configuration", func(t *testing.T) {
		// given
		configMapValueString := "false"
		istioCNIConfigMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      configMapCNI,
			Namespace: kymaNamespace,
		},
			Data: map[string]string{"wrongKey": configMapValueString},
		}
		client := k8sClientFake.NewSimpleClientset(istioCNIConfigMap)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)

		// when
		outputManifest, err := ApplyCNIConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		require.Equal(t, outputManifest, istioManifest)
	})
	t.Run("should not return merged configuration, when there is a Istio CM with the same configuration", func(t *testing.T) {
		// given
		configMapValueString := "true"
		istioCNIConfigMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      configMapCNI,
			Namespace: kymaNamespace,
		},
			Data: map[string]string{"enabled": configMapValueString},
		}
		client := k8sClientFake.NewSimpleClientset(istioCNIConfigMap)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)

		// when
		outputManifest, err := ApplyCNIConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		require.Equal(t, outputManifest, istioManifest)
	})
	t.Run("should default to the Istio Operator when ConfigMap is not on the cluster", func(t *testing.T) {
		// given
		client := k8sClientFake.NewSimpleClientset()
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)

		// when
		outputManifest, err := ApplyCNIConfiguration(ctx, provider, istioManifest, kubeConfig, log)

		// then
		require.NoError(t, err)
		require.Equal(t, outputManifest, istioManifest)
	})
}

func Test_GetActualCNIState(t *testing.T) {

	t.Run("should successfully check when CNI is enabled on the cluster", func(t *testing.T) {
		// given
		cniEnabled := true
		iop := istioOperator.IstioOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      istioOperatorName,
				Namespace: istioNamespace,
			},
			Spec: &operatorv1alpha1.IstioOperatorSpec{
				Components: &operatorv1alpha1.IstioComponentSetSpec{
					Cni: &operatorv1alpha1.ComponentSpec{
						Enabled: wrapperspb.Bool(cniEnabled),
					},
				},
			},
		}
		client := createClient(t, &iop)

		// when
		cniState, err := GetActualCNIState(client)

		// then
		require.NoError(t, err)
		require.Equal(t, cniEnabled, cniState)
	})
	t.Run("should return false if CNI is not set on the istio Operator", func(t *testing.T) {
		// given
		iop := istioOperator.IstioOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      istioOperatorName,
				Namespace: istioNamespace,
			},
			Spec: &operatorv1alpha1.IstioOperatorSpec{
				Components: &operatorv1alpha1.IstioComponentSetSpec{
					Cni: &operatorv1alpha1.ComponentSpec{},
				},
			},
		}
		client := createClient(t, &iop)

		// when
		cniState, err := GetActualCNIState(client)

		// then
		require.Equal(t, cniState, false)
		require.NoError(t, err)
	})
	t.Run("should return false if there is no Istio Operator on the cluster", func(t *testing.T) {
		// given
		scheme := runtime.NewScheme()
		err := istioOperator.SchemeBuilder.AddToScheme(scheme)
		require.NoError(t, err)
		client := dynamicfake.NewSimpleDynamicClient(scheme)

		// when
		cniState, err := GetActualCNIState(client)

		// then
		require.Equal(t, cniState, false)
		require.NoError(t, err)
	})
}
