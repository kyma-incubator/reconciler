package istio_test

import (
	"context"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	commandermocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

const istioctlMockCompleteVersion = `{
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

func Test_RunUninstallAction(t *testing.T) {

	t.Run("Istio uninstall should also delete namespace", func(t *testing.T) {

		wsf, _ := workspace.NewFactory(nil, "./test_files", log.NewLogger(true))

		actionContext := newActionContext(wsf)

		provider := clientset.DefaultProvider{}
		commanderMock := commandermocks.Commander{}
		commanderMock.On("Version", mock.Anything, mock.Anything).Return([]byte(istioctlMockCompleteVersion), nil)
		commanderMock.On("Uninstall", mock.Anything, mock.Anything).Return(nil)
		performer := actions.NewDefaultIstioPerformer(&commanderMock, nil, &provider)
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

func newActionContext(factory workspace.Factory) *service.ActionContext {
	provider, _ := chart.NewDefaultProvider(factory, log.NewLogger(true))
	kubeClient := newFakeKubeClient()

	logger := log.NewLogger(true)
	model := reconciler.Task{
		Component: "istio-configuration",
		Namespace: "istio-system",
		Version:   "0.0.0",
		Profile:   "production",
	}
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

	return mockClient
}
