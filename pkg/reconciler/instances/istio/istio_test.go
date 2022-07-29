package istio_test

import (
	"context"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	clientsetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/helpers"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	commandermocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	proxymocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy/mocks"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v12 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	istioctlMockCompleteVersion = `{
		"clientVersion": {
		  "version": "1.11.1",
		  "revision": "revision",
		  "golang_version": "go1.16.7",
		  "status": "Clean",
		  "tag": "1.11.1"
		},
		"meshVersion": [
		  {
			"Component": "pilot",
			"Info": {
			  "version": "1.11.1",
			  "revision": "revision",
			  "golang_version": "",
			  "status": "Clean",
			  "tag": "1.11.1"
			}
		  }
		],
		"dataPlaneVersion": [
		  {
			"ID": "id",
			"IstioVersion": "1.11.1"
		  }
		]
	  }`

	istioctlMockLatestVersion = `{
		"clientVersion": {
		  "version": "1.11.2",
		  "revision": "revision",
		  "golang_version": "go1.16.7",
		  "status": "Clean",
		  "tag": "1.11.2"
		},
		"meshVersion": [
		  {
			"Component": "pilot",
			"Info": {
			  "version": "1.12.4",
			  "revision": "revision",
			  "golang_version": "",
			  "status": "Clean",
			  "tag": "1.12.4"
			}
		  }
		],
		"dataPlaneVersion": [
		  {
			"ID": "id",
			"IstioVersion": "1.12.4"
		  }
		]
	  }`

	istioctlMockTooNewVersion = `{
		"clientVersion": {
		  "version": "1.09.2",
		  "revision": "revision",
		  "golang_version": "go1.16.7",
		  "status": "Clean",
		  "tag": "1.09.2"
		},
		"meshVersion": [
		  {
			"Component": "pilot",
			"Info": {
			  "version": "1.13.4",
			  "revision": "revision",
			  "golang_version": "",
			  "status": "Clean",
			  "tag": "1.13.4"
			}
		  }
		],
		"dataPlaneVersion": [
		  {
			"ID": "id",
			"IstioVersion": "1.13.4"
		  }
		]
	  }`

	istioctlMockDataPlanePilotMismatchVersion = `{
		"clientVersion": {
		  "version": "1.11.2",
		  "revision": "revision",
		  "golang_version": "go1.16.7",
		  "status": "Clean",
		  "tag": "1.11.2"
		},
		"meshVersion": [
		  {
			"Component": "pilot",
			"Info": {
			  "version": "1.13.4",
			  "revision": "revision",
			  "golang_version": "",
			  "status": "Clean",
			  "tag": "1.13.4"
			}
		  }
		],
		"dataPlaneVersion": [
		  {
			"ID": "id",
			"IstioVersion": "1.13.5"
		  }
		]
	  }`
)

// TODO(piotrkpc): here we are testing particular action's behaviour not Istio reconciler. Consider moving those to action_test.go
func Test_RunUpdateAction(t *testing.T) {

	performerCreatorFn := func(p *actions.DefaultIstioPerformer) func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
		return func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
			return p, nil
		}
	}

	wsf, _ := chart.NewFactory(nil, "./test_files", log.NewLogger(true))
	model := reconciler.Task{
		Component: "istio",
		Namespace: "istio-system",
		Version:   "1.11.2",
		Profile:   "production",
	}
	actionContext := newActionContext(wsf, model)

	t.Run("Istio update should permit one minor downgrade", func(t *testing.T) {
		// given
		providerMock := clientsetmocks.Provider{}
		providerMock.On("RetrieveFrom", mock.Anything, mock.Anything).Return(fake.NewSimpleClientset(), nil)
		commanderMock := commandermocks.Commander{}
		commanderMock.On("Version", mock.Anything, mock.Anything).Return([]byte(istioctlMockLatestVersion), nil)
		commanderMock.On("Upgrade", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &commanderMock}

		proxy := proxymocks.IstioProxyReset{}
		proxy.On("Run", mock.Anything).Return(nil)
		performer := actions.NewDefaultIstioPerformer(cmdResolver, &proxy, &providerMock)

		action := istio.NewIstioMainReconcileAction(performerCreatorFn(performer))

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		commanderMock.AssertCalled(t, "Version", mock.Anything, mock.Anything)
		commanderMock.AssertCalled(t, "Upgrade", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Istio update should NOT permit more than one minor downgrade", func(t *testing.T) {
		// given
		provider := clientset.DefaultProvider{}
		commanderMock := commandermocks.Commander{}
		commanderMock.On("Version", mock.Anything, mock.Anything).Return([]byte(istioctlMockTooNewVersion), nil)
		cmdResolver := TestCommanderResolver{cmder: &commanderMock}
		performer := actions.NewDefaultIstioPerformer(cmdResolver, nil, &provider)

		action := istio.NewStatusPreAction(performerCreatorFn(performer))

		// when
		err := action.Run(actionContext)

		// then
		require.Contains(t, err.Error(), "difference between versions exceeds one minor version")
		commanderMock.AssertCalled(t, "Version", mock.Anything, mock.Anything)
	})

	t.Run("Istio update should be allowed when there is data plane and pilot version mismatch if the data plane is consistent", func(t *testing.T) {
		// given
		provider := clientset.DefaultProvider{}
		commanderMock := commandermocks.Commander{}
		commanderMock.On("Version", mock.Anything, mock.Anything).Return([]byte(istioctlMockDataPlanePilotMismatchVersion), nil)
		cmdResolver := TestCommanderResolver{cmder: &commanderMock}
		performer := actions.NewDefaultIstioPerformer(cmdResolver, nil, &provider)
		action := istio.NewStatusPreAction(performerCreatorFn(performer))

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		commanderMock.AssertCalled(t, "Version", mock.Anything, mock.Anything)
	})

}

func Test_RunUninstallAction(t *testing.T) {

	performerCreatorFn := func(p *actions.DefaultIstioPerformer) func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
		return func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
			return p, nil
		}
	}

	t.Run("Istio uninstall should also delete namespace", func(t *testing.T) {
		// given
		wsf, _ := chart.NewFactory(nil, "./test_files", log.NewLogger(true))
		model := reconciler.Task{
			Component: "istio",
			Namespace: "istio-system",
			Version:   "0.0.0",
			Profile:   "production",
		}
		actionContext := newActionContext(wsf, model)
		provider := clientset.DefaultProvider{}
		commanderMock := commandermocks.Commander{}
		commanderMock.On("Version", mock.Anything, mock.Anything).Return([]byte(istioctlMockCompleteVersion), nil)
		commanderMock.On("Uninstall", mock.Anything, mock.Anything).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &commanderMock}

		performer := actions.NewDefaultIstioPerformer(cmdResolver, nil, &provider)
		action := istio.NewUninstallAction(performerCreatorFn(performer))

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		commanderMock.AssertCalled(t, "Version", mock.Anything, mock.Anything)
		commanderMock.AssertCalled(t, "Uninstall", mock.Anything, mock.Anything)

		// istio-system namespace should be deleted
		fakeClient, _ := actionContext.KubeClient.Clientset()
		ns, nserror := fakeClient.CoreV1().Namespaces().Get(context.TODO(), "istio-system", metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})

		t.Log(ns)
		require.Error(t, nserror)
	})

}

func newActionContext(factory chart.Factory, model reconciler.Task) *service.ActionContext {
	provider, _ := chart.NewDefaultProvider(factory, log.NewLogger(true))
	kubeClient := newFakeKubeClient()

	logger := log.NewLogger(true)
	return &service.ActionContext{
		KubeClient:       kubeClient,
		Context:          context.Background(),
		WorkspaceFactory: factory,
		Logger:           logger,
		ChartProvider:    provider,
		Task:             &model,
	}
}

func newFakeKubeClient() *k8smocks.Client {
	mockClient := &k8smocks.Client{}
	fakeClient := fake.NewSimpleClientset(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-system",
		},
	},
		&v12.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
			Webhooks: []v12.MutatingWebhook{
				{
					Name: "auto.sidecar-injector.istio.io",
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: nil,
					},
				},
			},
		},
	)
	mockClient.On("Clientset").Return(fakeClient, nil)
	mockClient.On("Kubeconfig").Return("kubeconfig")
	mockClient.On("Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockClient.On("CoreV1").Return(nil)
	mockClient.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockClient.On("PatchUsingStrategy", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	return mockClient
}

type TestCommanderResolver struct {
	err   error
	cmder istioctl.Commander
}

func (tcr TestCommanderResolver) GetCommander(_ helpers.HelperVersion) (istioctl.Commander, error) {
	if tcr.err != nil {
		return nil, tcr.err
	}

	return tcr.cmder, nil
}

func TestIstioReconciler(t *testing.T) {
	istioReconciler, err := service.GetReconciler(istio.ReconcilerNameIstio)

	t.Run("should register Istio reconciler", func(t *testing.T) {
		require.NoError(t, err)
		require.NotNil(t, istioReconciler)
	})
}
