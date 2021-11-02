package istio

import (
	"context"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	actionsmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	workspacemocks "github.com/kyma-incubator/reconciler/pkg/reconciler/workspace/mocks"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	istioManifest = `---
apiVersion: version/v1
kind: Kind1
metadata:
  namespace: namespace
  name: name
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  namespace: namespace
  name: name
---
apiVersion: version/v2
kind: Kind2
metadata:
  namespace: namespace
  name: name
`

	istioManifestWithoutIstioOperator = `---
apiVersion: version/v1
kind: Kind1
metadata:
  namespace: namespace
  name: name
---
apiVersion: version/v2
kind: Kind2
metadata:
  namespace: namespace
  name: name
`
)

func Test_ReconcileAction_Run(t *testing.T) {

	t.Run("should not perform any istio action when provider returned an error ", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(nil, errors.New("Provider error"))
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Provider error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertNotCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform any istio action when commander version returned an error ", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(actions.IstioVersion{}, errors.New("Version error"))
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Version error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio install action when istio was not detected on the cluster and istio install returned an error", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		noIstioOnTheCluster := actions.IstioVersion{
			ClientVersion:    "1.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(noIstioOnTheCluster, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(errors.New("Perfomer Install error"))
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Perfomer Install error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio install action when istio was not detected on the cluster and istio patch returned an error", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		noIstioOnTheCluster := actions.IstioVersion{
			ClientVersion:    "1.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(noIstioOnTheCluster, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.KubeClient, actionContext.Logger).Return(errors.New("Performer Patch error"))
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Performer Patch error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "PatchMutatingWebhook", mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should perform istio install action when istio was not detected on the cluster", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		noIstioOnTheCluster := actions.IstioVersion{
			ClientVersion:    "1.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(noIstioOnTheCluster, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.KubeClient, actionContext.Logger).Return(nil)
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "PatchMutatingWebhook", mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio update action when istio was detected on the cluster and client version is lower than target version", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooLowClientVersion := actions.IstioVersion{
			ClientVersion:    "1.0",
			TargetVersion:    "1.2",
			PilotVersion:     "1.1",
			DataPlaneVersion: "1.1",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowClientVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.KubeClient, actionContext.Logger).Return(nil)
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio update action when istio was detected on the cluster and downgrade is detected", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooLowClientVersion := actions.IstioVersion{
			ClientVersion:    "1.2",
			TargetVersion:    "0.9",
			PilotVersion:     "1.1",
			DataPlaneVersion: "1.1",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowClientVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.KubeClient, actionContext.Logger).Return(nil)
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio update action when istio was detected on the cluster and more than one minor upgrade was detected", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooLowClientVersion := actions.IstioVersion{
			ClientVersion:    "1.3",
			TargetVersion:    "1.3",
			PilotVersion:     "1.1",
			DataPlaneVersion: "1.1",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowClientVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.KubeClient, actionContext.Logger).Return(nil)
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should return error when istio was updated but proxies were not reset", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooLowClientVersion := actions.IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowClientVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.KubeClient, actionContext.Logger).Return(nil)
		performer.On("Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger).Return(errors.New("Proxy reset error"))
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Proxy reset error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not return error when istio was reconciled to the same version and proxies reset was successful", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooLowClientVersion := actions.IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.2.0",
			DataPlaneVersion: "1.2.0",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowClientVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.KubeClient, actionContext.Logger).Return(nil)
		performer.On("Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger).Return(nil)
		action := ReconcileAction{performer: &performer}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "ResetProxy", mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

}

func newFakeServiceContext(factory workspace.Factory, provider chart.Provider, client kubernetes.Client) *service.ActionContext {
	logger := log.NewLogger(true)
	model := reconciler.Task{
		Component: "component",
		Namespace: "namespace",
		Version:   "version",
		Profile:   "profile",
	}

	return &service.ActionContext{
		KubeClient:       client,
		Context:          context.Background(),
		WorkspaceFactory: factory,
		Logger:           logger,
		ChartProvider:    provider,
		Task:             &model,
	}
}

func newFakeKubeClient() *k8smocks.Client {
	mockClient := &k8smocks.Client{}
	mockClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
	mockClient.On("Kubeconfig").Return("kubeconfig")
	mockClient.On("Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	return mockClient
}

func Test_canInstall(t *testing.T) {
	t.Run("should install when client and pilot versions are empty", func(t *testing.T) {
		//given
		randomVersion := actions.IstioVersion{
			ClientVersion:    "1.9.2",
			TargetVersion:    "",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		//when
		got := canInstall(randomVersion)

		//then
		require.Equal(t, true, got)
	})

	t.Run("should update when client and pilot versions values are not empty", func(t *testing.T) {
		//given
		randomVersion := actions.IstioVersion{
			ClientVersion:    "1.11.2",
			TargetVersion:    "",
			PilotVersion:     "1.11.1",
			DataPlaneVersion: "1.11.1",
		}

		//when
		got := canInstall(randomVersion)

		//then
		require.Equal(t, false, got)
	})
}

func Test_canUpdate(t *testing.T) {
	logger := log.NewLogger(true)

	t.Run("should not allow update when client version is more than one minor behind the target version", func(t *testing.T) {
		//given
		version := actions.IstioVersion{
			ClientVersion:    "1.0.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.0.0",
			DataPlaneVersion: "1.0.0",
		}

		//when
		result := canUpdate(version, logger)

		//then
		require.False(t, result)
	})

	t.Run("should not allow update when downgrade scenario is detected for pilot", func(t *testing.T) {
		//given
		version := actions.IstioVersion{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.1.0",
			PilotVersion:     "1.2.0",
			DataPlaneVersion: "1.1.0",
		}

		//when
		result := canUpdate(version, logger)

		//then
		require.False(t, result)
	})

	t.Run("should not allow update when downgrade scenario is detected for data plane", func(t *testing.T) {
		//given
		version := actions.IstioVersion{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.1.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.2.0",
		}

		//when
		result := canUpdate(version, logger)

		//then
		require.False(t, result)
	})

	t.Run("should not allow update when more than one minor upgrade is detected for pilot", func(t *testing.T) {
		//given
		version := actions.IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.0.0",
			DataPlaneVersion: "1.1.0",
		}

		//when
		result := canUpdate(version, logger)

		//then
		require.False(t, result)
	})

	t.Run("should not allow update when more than one minor upgrade is detected for data plane", func(t *testing.T) {
		//given
		version := actions.IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.0.0",
		}

		//when
		result := canUpdate(version, logger)

		//then
		require.False(t, result)
	})

	t.Run("should allow update when less than one minor upgrade is detected for pilot and data plane ", func(t *testing.T) {
		//given
		version := actions.IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}

		//when
		result := canUpdate(version, logger)

		//then
		require.True(t, result)
	})

	t.Run("should allow update when all versions match", func(t *testing.T) {
		//given
		version := actions.IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.2.0",
			DataPlaneVersion: "1.2.0",
		}

		//when
		result := canUpdate(version, logger)

		//then
		require.True(t, result)
	})
}

func TestIsMismatchPresent(t *testing.T) {
	t.Run("Different Pilot and DataPlane versions is a mismatch", func(t *testing.T) {
		//given
		differentVersions := actions.IstioVersion{
			ClientVersion:    "1.11.2",
			PilotVersion:     "1.11.1",
			DataPlaneVersion: "1.11.2",
		}

		//when
		got := isMismatchPresent(differentVersions)

		//then
		require.Equal(t, true, got)
	})

	t.Run("Same Pilot and DataPlane versions is not a mismatch", func(t *testing.T) {
		//given
		sameVersions := actions.IstioVersion{
			ClientVersion:    "1.11.2",
			PilotVersion:     "1.11.2",
			DataPlaneVersion: "1.11.2",
		}

		//when
		got := isMismatchPresent(sameVersions)

		//then
		require.Equal(t, false, got)
	})
}

func Test_generateNewManifestWithoutIstioOperatorFrom(t *testing.T) {

	t.Run("should generate empty manifest from empty input manifest", func(t *testing.T) {
		// when
		result, err := generateNewManifestWithoutIstioOperatorFrom("")

		// then
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("should return manifest without IstioOperator kind if it was not present ", func(t *testing.T) {
		// given
		require.Contains(t, istioManifestWithoutIstioOperator, "Kind1")
		require.Contains(t, istioManifestWithoutIstioOperator, "Kind2")
		require.NotContains(t, istioManifestWithoutIstioOperator, "IstioOperator")

		// when
		result, err := generateNewManifestWithoutIstioOperatorFrom(istioManifestWithoutIstioOperator)

		// then
		require.NoError(t, err)
		require.Contains(t, result, "Kind1")
		require.Contains(t, result, "Kind2")
		require.NotContains(t, result, "IstioOperator")
	})

	t.Run("should return manifest without IstioOperator kind if it was present", func(t *testing.T) {
		// given
		require.Contains(t, istioManifest, "Kind1")
		require.Contains(t, istioManifest, "Kind2")
		require.Contains(t, istioManifest, "IstioOperator")

		// when
		result, err := generateNewManifestWithoutIstioOperatorFrom(istioManifest)

		// then
		require.NoError(t, err)
		require.Contains(t, result, "Kind1")
		require.Contains(t, result, "Kind2")
		require.NotContains(t, result, "IstioOperator")
	})

}
