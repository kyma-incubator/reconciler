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
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	commandermocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	proxymocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy/mocks"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func Test_RunUpdateAction(t *testing.T) {
	wsf, _ := chart.NewFactory(nil, "./test_files", log.NewLogger(true))
	model := reconciler.Task{
		Component: "istio-configuration",
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
		action := istio.NewReconcileAction(performer)

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
		commanderMock.On("Upgrade", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &commanderMock}
		performer := actions.NewDefaultIstioPerformer(cmdResolver, nil, &provider)
		action := istio.NewReconcileAction(performer)

		// when
		err := action.Run(actionContext)

		// then
		require.EqualError(t, err, "Istio could not be updated since the binary version: 1.09.2 is not compatible with the target version: 1.11.2 - the difference between versions exceeds one minor version")
		commanderMock.AssertCalled(t, "Version", mock.Anything, mock.Anything)
		commanderMock.AssertNotCalled(t, "Upgrade", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Istio update should return an error when there is data plane and pilot version mismatch", func(t *testing.T) {
		// given
		provider := clientset.DefaultProvider{}
		commanderMock := commandermocks.Commander{}
		commanderMock.On("Version", mock.Anything, mock.Anything).Return([]byte(istioctlMockDataPlanePilotMismatchVersion), nil)
		commanderMock.On("Upgrade", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &commanderMock}
		performer := actions.NewDefaultIstioPerformer(cmdResolver, nil, &provider)
		action := istio.NewReconcileAction(performer)

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.EqualError(t, err, "Istio components version mismatch detected: pilot version: 1.13.4, data plane version: 1.13.5")
		commanderMock.AssertCalled(t, "Version", mock.Anything, mock.Anything)
		commanderMock.AssertNotCalled(t, "Upgrade", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Istio update should return an error when there is data plane and pilot version mismatch", func(t *testing.T) {
		// given
		provider := clientset.DefaultProvider{}
		commanderMock := commandermocks.Commander{}
		commanderMock.On("Version", mock.Anything, mock.Anything).Return([]byte(istioctlMockDataPlanePilotMismatchVersion), nil)
		commanderMock.On("Upgrade", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &commanderMock}
		performer := actions.NewDefaultIstioPerformer(cmdResolver, nil, &provider)
		action := istio.NewReconcileAction(performer)

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.EqualError(t, err, "Istio components version mismatch detected: pilot version: 1.13.4, data plane version: 1.13.5")
		commanderMock.AssertCalled(t, "Version", mock.Anything, mock.Anything)
		commanderMock.AssertNotCalled(t, "Upgrade", mock.Anything, mock.Anything, mock.Anything)
	})
}

func Test_RunUninstallAction(t *testing.T) {
	t.Run("Istio uninstall should also delete namespace", func(t *testing.T) {
		// given
		wsf, _ := chart.NewFactory(nil, "./test_files", log.NewLogger(true))
		model := reconciler.Task{
			Component: "istio-configuration",
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
		action := istio.NewUninstallAction(performer)

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		commanderMock.AssertCalled(t, "Version", mock.Anything, mock.Anything)
		commanderMock.AssertCalled(t, "Uninstall", mock.Anything, mock.Anything)

		//istio-system namespace should be deleted
		fakeClient, _ := actionContext.KubeClient.Clientset()
		_, nserror := fakeClient.CoreV1().Namespaces().Get(context.TODO(), "istio-system", metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
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
	})
	mockClient.On("Clientset").Return(fakeClient, nil)
	mockClient.On("Kubeconfig").Return("kubeconfig")
	mockClient.On("Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockClient.On("CoreV1").Return(nil)
	mockClient.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	mockClient.On("PatchUsingStrategy", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	return mockClient
}

type TestCommanderResolver struct {
	err   error
	cmder istioctl.Commander
}

func (tcr TestCommanderResolver) GetCommander(version istioctl.Version) (istioctl.Commander, error) {
	if tcr.err != nil {
		return nil, tcr.err
	} else {
		return tcr.cmder, nil
	}
}
