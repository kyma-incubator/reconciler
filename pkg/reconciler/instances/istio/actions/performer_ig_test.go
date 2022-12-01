package actions

import (
	"context"
	"testing"

	"github.com/kyma-project/istio/operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	testutils "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions/test-utils"
	clientsetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset/mocks"
	istioctlmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	proxymocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy/mocks"
	"github.com/stretchr/testify/mock"
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

func Test_DefaultIstioPerfomer_UpdateIGRestart(t *testing.T) {
	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not restart IG when no config changed", func(t *testing.T) {
		// given
		ctrlClientSameConfig := testutils.GetIGClient(t, TestConfigMap)
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

		err := ctrlClientSameConfig.Create(context.TODO(), istioCR)
		require.NoError(t, err)

		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.Anything).Return(ctrlClientSameConfig, nil)
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(fake.NewSimpleClientset(), nil)

		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err = wrapper.Update(context.TODO(), kubeConfig, istioManifest, "1.2.3", log)

		// then
		require.NoError(t, err)

		dep := appsv1.Deployment{}
		err = ctrlClientSameConfig.Get(context.TODO(), types.NamespacedName{Namespace: testutils.DepNamespace, Name: testutils.DepName}, &dep)
		require.NoError(t, err)
		require.Empty(t, dep.Spec.Template.Annotations["reconciler.kyma-project.io/lastRestartDate"])
	})

	t.Run("should restart IG when config changed", func(t *testing.T) {
		// given
		ctrlClientDiffConfig := testutils.GetIGClient(t, TestConfigMap)
		numTrustedProxies := 2

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

		err := ctrlClientDiffConfig.Create(context.TODO(), istioCR)
		require.NoError(t, err)

		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		provider.On("GetIstioClient", mock.Anything).Return(ctrlClientDiffConfig, nil)
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(fake.NewSimpleClientset(), nil)

		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err = wrapper.Update(context.TODO(), kubeConfig, istioManifest, "1.2.3", log)

		// then
		require.NoError(t, err)

		dep := appsv1.Deployment{}
		err = ctrlClientDiffConfig.Get(context.TODO(), types.NamespacedName{Namespace: testutils.DepNamespace, Name: testutils.DepName}, &dep)
		require.NoError(t, err)
		require.NotEmpty(t, dep.Spec.Template.Annotations["reconciler.kyma-project.io/lastRestartDate"])
	})
}
