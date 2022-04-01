package istio

import (
	"context"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	actionsmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/manifest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	unequal       = -1
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

func Test_newVersionHelperFrom(t *testing.T) {

	t.Run("should return an error when input string contains less than three numbers", func(t *testing.T) {
		// when
		_, err := newHelperVersionFrom("1.2.")

		// then
		require.Error(t, err)
	})

	t.Run("should return an error when input string contains less than two dots", func(t *testing.T) {
		// when
		_, err := newHelperVersionFrom("1.23")

		// then
		require.Error(t, err)
	})

	t.Run("should return an error when input string contains three numbers, two dots and prefix", func(t *testing.T) {
		// when
		_, err := newHelperVersionFrom("prefix-1.2.3")

		// then
		require.Error(t, err)
	})

	t.Run("should return no error when input string contains three numbers, two dots, prefix and suffix", func(t *testing.T) {
		// when
		_, err := newHelperVersionFrom("prefix-1.2.3-suffix")

		// then
		require.Error(t, err)
	})

	t.Run("should return an error when input string contains three numbers, two dots and text in between", func(t *testing.T) {
		// when
		_, err := newHelperVersionFrom("1.text2.3")

		// then
		require.Error(t, err)
	})

	t.Run("should return no error when input string contains three numbers and two dots", func(t *testing.T) {
		// when
		_, err := newHelperVersionFrom("1.2.3")

		// then
		require.NoError(t, err)
	})

	t.Run("should return no error when input string contains three numbers, two dots and suffix", func(t *testing.T) {
		// when
		_, err := newHelperVersionFrom("1.2.3-suffix")

		// then
		require.NoError(t, err)
	})

}

func Test_helperVersion_compare(t *testing.T) {

	t.Run("should return true when helper versions are of different numbers", func(t *testing.T) {
		// given
		v1, err := newHelperVersionFrom("1.2.3")
		require.NoError(t, err)
		v2, err := newHelperVersionFrom("4.5.6")
		require.NoError(t, err)

		// when
		result := v1.compare(v2)

		// then
		require.Equal(t, unequal, result)
	})

	t.Run("should return true when helper versions are of equal numbers", func(t *testing.T) {
		// given
		v1, err := newHelperVersionFrom("1.2.3")
		require.NoError(t, err)
		v2, err := newHelperVersionFrom("1.2.3")
		require.NoError(t, err)

		// when
		result := v1.compare(v2)

		// then
		require.Zero(t, result)
	})

	t.Run("should return true when helper versions are of equal numbers and one has suffix", func(t *testing.T) {
		// given
		v1, err := newHelperVersionFrom("1.2.3-suffix")
		require.NoError(t, err)
		v2, err := newHelperVersionFrom("1.2.3")
		require.NoError(t, err)

		// when
		result := v1.compare(v2)

		// then
		require.Zero(t, result)
	})

	t.Run("should return true when helper versions are of equal numbers and both have different suffixes", func(t *testing.T) {
		// given
		v1, err := newHelperVersionFrom("1.2.3-suffix1")
		require.NoError(t, err)
		v2, err := newHelperVersionFrom("1.2.3-suffix2")
		require.NoError(t, err)

		// when
		result := v1.compare(v2)

		// then
		require.Zero(t, result)
	})

}

func TestStatusPreAction_Run(t *testing.T) {

	performerCreatorFn := func(p actions.IstioPerformer) bootstrapIstioPerformer {
		return func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
			return p, nil
		}
	}

	t.Run("should not perform istio actions when istio was detected on the cluster and client version is lower than target version", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooLowClientVersion := actions.IstioStatus{
			ClientVersion:    "1.0",
			TargetVersion:    "1.2",
			PilotVersion:     "1.1",
			DataPlaneVersion: "1.1",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowClientVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)

		action := StatusPreAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.EqualError(t, err, "Istio could not be updated since the binary version: 1.0 is not compatible with the target version: 1.2 - the difference between versions exceeds one minor version")
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

}

func Test_ReconcileAction_Run(t *testing.T) {
	// TODO: rewrite
	//performerCreatorFn := func(p actions.IstioPerformer) bootstrapIstioPerformer {
	//	return func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
	//		return p, nil
	//	}
	//}

	performerCreatorErrorFn := func(p actions.IstioPerformer) bootstrapIstioPerformer {
		return func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
			return p, errors.New("Performer error")
		}
	}

	t.Run("should not perform any istio action when performer returned error", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		action := MainReconcileAction{performerCreatorErrorFn(&performer)}

		//when
		err := action.Run(actionContext)

		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Performer error")
		provider.AssertNotCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertNotCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
	})
	// TODO: rewrite
	//t.Run("should patch webhook even if provider returned error", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	action := MainReconcileAction{performerCreatorErrorFn(&performer)}
	//
	//	//when
	//	err := action.Run(actionContext)
	//
	//	//then
	//	require.Error(t, err)
	//	require.Contains(t, err.Error(), "Performer error")
	//	provider.AssertNotCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
	//	performer.AssertNotCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	//	performer.AssertCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
	//})
	// TODO: rewrite
	////====================================================================
	//t.Run("should not perform any istio action when provider returned an error ", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	action := MainReconcileAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	err := action.Run(actionContext)
	//
	//	// then
	//	require.Error(t, err)
	//	require.Contains(t, err.Error(), "Provider error")
	//	provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
	//	performer.AssertNotCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	//
	//	// performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(errors.New("Performer Patch error"))
	//	performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
	//
	//	performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
	//})
	//
	//t.Run("should not perform any istio action when commander version returned an error ", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(actions.IstioStatus{}, errors.New("Version error"))
	//
	//	action := MainReconcileAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	err := action.Run(actionContext)
	//
	//	// then
	//	require.Error(t, err)
	//	require.Contains(t, err.Error(), "Version error")
	//	provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
	//	performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	//	performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
	//})
	//
	//t.Run("should not perform istio install action when istio was not detected on the cluster and istio install returned an error", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	noIstioOnTheCluster := actions.IstioStatus{
	//		ClientVersion:    "1.0.0",
	//		TargetVersion:    "1.0.0",
	//		PilotVersion:     "",
	//		DataPlaneVersion: "",
	//	}
	//	performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(noIstioOnTheCluster, nil)
	//	performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
	//		Return(errors.New("Perfomer Install error"))
	//
	//	action := MainReconcileAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	err := action.Run(actionContext)
	//
	//	// then
	//	require.Error(t, err)
	//	require.Contains(t, err.Error(), "Perfomer Install error")
	//	provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
	//	performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	//	performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
	//})
	//
	//t.Run("should not perform istio install action when istio was not detected on the cluster and istio patch returned an error", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	provider := chartmocks.Provider{}
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	noIstioOnTheCluster := actions.IstioStatus{
	//		ClientVersion:    "1.0.0",
	//		TargetVersion:    "1.0.0",
	//		PilotVersion:     "",
	//		DataPlaneVersion: "",
	//	}
	//	performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(noIstioOnTheCluster, nil)
	//	performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(errors.New("Performer Patch error"))
	//
	//	//action := MutatingWebhookPostAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	//err := action.Run(actionContext)
	//
	//	// then
	//	//require.Error(t, err)
	//	//require.Contains(t, err.Error(), "Performer Patch error")
	//	performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertCalled(t, "PatchMutatingWebhook", mock.Anything, mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
	//})
	//
	//t.Run("should perform istio install action when istio was not detected on the cluster", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	noIstioOnTheCluster := actions.IstioStatus{
	//		ClientVersion:    "1.0.0",
	//		TargetVersion:    "1.0.0",
	//		PilotVersion:     "",
	//		DataPlaneVersion: "",
	//	}
	//	performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(noIstioOnTheCluster, nil)
	//	performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
	//
	//	action := MainReconcileAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	err := action.Run(actionContext)
	//
	//	// then
	//	require.NoError(t, err)
	//	provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
	//	performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//})
	//
	//t.Run("should not perform istio update action when istio was detected on the cluster and downgrade is detected", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
	//		ResourceDir: "./test_files/resources/",
	//	}, nil)
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	tooHighPilotAndDataPlaneVersion := actions.IstioStatus{
	//		ClientVersion:    "0.9.0",
	//		TargetVersion:    "0.9.0",
	//		PilotVersion:     "1.1.0",
	//		DataPlaneVersion: "1.1.0",
	//	}
	//	performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooHighPilotAndDataPlaneVersion, nil)
	//	performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
	//	performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(nil)
	//
	//	action := MainReconcileAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	err := action.Run(actionContext)
	//
	//	// then
	//	require.Error(t, err)
	//	provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
	//	performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
	//})
	//
	//t.Run("should not perform istio update action when istio was detected on the cluster and more than one minor upgrade was detected", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
	//		ResourceDir: "./test_files/resources/",
	//	}, nil)
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	tooLowPilotAndDataPlaneVersion := actions.IstioStatus{
	//		ClientVersion:    "1.3.0",
	//		TargetVersion:    "1.3.0",
	//		PilotVersion:     "1.1.0",
	//		DataPlaneVersion: "1.1.0",
	//	}
	//	performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowPilotAndDataPlaneVersion, nil)
	//	performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
	//	performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(nil)
	//
	//	action := MainReconcileAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	err := action.Run(actionContext)
	//
	//	// then
	//	require.Error(t, err)
	//	provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
	//	performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
	//})
	//
	//t.Run("should return error when istio was updated but proxies were not reset", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
	//		ResourceDir: "./test_files/resources/",
	//	}, nil)
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	istioVersion := actions.IstioStatus{
	//		ClientVersion:    "1.2.0",
	//		TargetVersion:    "1.2.0",
	//		PilotVersion:     "1.1.0",
	//		DataPlaneVersion: "1.1.0",
	//	}
	//	performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(istioVersion, nil)
	//	performer.On("ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(errors.New("Proxy reset error"))
	//
	//	action := ProxyResetPostAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	err := action.Run(actionContext)
	//
	//	// then
	//	require.Error(t, err)
	//	require.Contains(t, err.Error(), "Proxy reset error")
	//	performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger)
	//})
	//
	//t.Run("should not return error when istio was reconciled to the same version and proxies reset was successful", func(t *testing.T) {
	//	// given
	//	factory := chartmocks.Factory{}
	//	factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
	//		ResourceDir: "./test_files/resources/",
	//	}, nil)
	//	provider := chartmocks.Provider{}
	//	provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
	//	kubeClient := newFakeKubeClient()
	//	actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
	//	performer := actionsmocks.IstioPerformer{}
	//	istioVersion := actions.IstioStatus{
	//		ClientVersion:    "1.2.0",
	//		TargetVersion:    "1.2.0",
	//		PilotVersion:     "1.2.0",
	//		DataPlaneVersion: "1.2.0",
	//	}
	//	performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(istioVersion, nil)
	//	performer.On("Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
	//
	//	action := MainReconcileAction{performerCreatorFn(&performer)}
	//
	//	// when
	//	err := action.Run(actionContext)
	//
	//	// then
	//	require.NoError(t, err)
	//	provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
	//	performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//	performer.AssertCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	//})
}

func Test_ReconcileIstioConfigurationAction_Run(t *testing.T) {

	performerCreatorFn := func(p actions.IstioPerformer) bootstrapIstioPerformer {
		return func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
			return p, nil
		}
	}

	t.Run("should not perform any istio-configuration action when provider returned an error ", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(nil, errors.New("Provider error"))
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Provider error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertNotCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform any istio-configuration action when commander version returned an error ", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(actions.IstioStatus{}, errors.New("Version error"))

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Version error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio-configuration install action when istio was not detected on the cluster and istio install returned an error", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		noIstioOnTheCluster := actions.IstioStatus{
			ClientVersion:    "1.0.0",
			TargetVersion:    "1.0.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(noIstioOnTheCluster, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(errors.New("Perfomer Install error"))

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Perfomer Install error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.AnythingOfType("kubernetes.Client"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio-configuration install action when istio was not detected on the cluster and istio patch returned an error", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		noIstioOnTheCluster := actions.IstioStatus{
			ClientVersion:    "1.0.0",
			TargetVersion:    "1.0.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(noIstioOnTheCluster, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(errors.New("Performer Patch error"))

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Performer Patch error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "PatchMutatingWebhook", mock.Anything, mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
	})

	t.Run("should perform istio-configuration install action when istio was not detected on the cluster", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		noIstioOnTheCluster := actions.IstioStatus{
			ClientVersion:    "1.0.0",
			TargetVersion:    "1.0.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(noIstioOnTheCluster, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(nil)

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "PatchMutatingWebhook", mock.Anything, mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio-configuration update action when istio was detected on the cluster and client version is lower than target version", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooLowClientVersion := actions.IstioStatus{
			ClientVersion:    "1.0",
			TargetVersion:    "1.2",
			PilotVersion:     "1.1",
			DataPlaneVersion: "1.1",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowClientVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(nil)

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio-configuration update action when istio was detected on the cluster and downgrade is detected", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooHighPilotAndDataPlaneVersion := actions.IstioStatus{
			ClientVersion:    "0.9.0",
			TargetVersion:    "0.9.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooHighPilotAndDataPlaneVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(nil)

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not perform istio-configuration update action when istio was detected on the cluster and more than one minor upgrade was detected", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		tooLowPilotAndDataPlaneVersion := actions.IstioStatus{
			ClientVersion:    "1.3.0",
			TargetVersion:    "1.3.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(tooLowPilotAndDataPlaneVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(nil)

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "PatchMutatingWebhook", mock.AnythingOfType("context.Context"), mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("IstioVersion"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should return error when istio-configuration was updated but proxies were not reset", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(istioVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(nil)
		performer.On("Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(errors.New("Proxy reset error"))

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Proxy reset error")
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "PatchMutatingWebhook", mock.Anything, mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger)
		kubeClient.AssertNotCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not return error when istio-configuration was reconciled to the same version and proxies reset was successful", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{
			ResourceDir: "./test_files/resources/",
		}, nil)
		provider := chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.2.0",
			DataPlaneVersion: "1.2.0",
		}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(istioVersion, nil)
		performer.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("PatchMutatingWebhook", actionContext.Context, actionContext.KubeClient, actionContext.Logger).Return(nil)
		performer.On("Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)
		performer.On("ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger).Return(nil)

		action := ReconcileIstioConfigurationAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "RenderManifest", mock.AnythingOfType("*chart.Component"))
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "PatchMutatingWebhook", mock.Anything, mock.Anything, mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "ResetProxy", actionContext.Context, mock.AnythingOfType("string"), mock.AnythingOfType("string"), actionContext.Logger)
		kubeClient.AssertCalled(t, "Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func newFakeServiceContext(factory chart.Factory, provider chart.Provider, client kubernetes.Client) *service.ActionContext {
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

func Test_UninstallAction(t *testing.T) {
	performerCreatorFn := func(p actions.IstioPerformer) bootstrapIstioPerformer {
		return func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
			return p, nil
		}
	}

	noIstioOnTheCluster := actions.IstioStatus{
		ClientVersion:    "1.0",
		PilotVersion:     "",
		DataPlaneVersion: "",
	}

	istioAvailable := actions.IstioStatus{
		ClientVersion:    "1.0",
		PilotVersion:     "1.0",
		DataPlaneVersion: "1.0",
	}

	t.Run("should perform istio uninstall action when istio is available", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		kubeClient := newFakeKubeClient()
		kubeClient.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType(
			"string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(istioAvailable, nil)
		performer.On("Uninstall", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(&chart.Manifest{}, nil)

		action := UninstallAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.
			AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertCalled(t, "Uninstall", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should not perform istio uninstall action when istio was not detected on the cluster", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType(
			"string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(noIstioOnTheCluster, nil)

		action := UninstallAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	})

	t.Run("should not perform istio uninstall action when there is an error detecting istio version", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		performer := actionsmocks.IstioPerformer{}
		performer.On("Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType(
			"string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(noIstioOnTheCluster, errors.New("error in detecting istio version"))

		action := UninstallAction{performerCreatorFn(&performer)}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Could not fetch Istio version: error in detecting istio version")
		performer.AssertCalled(t, "Version", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		performer.AssertNotCalled(t, "Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	})

}
func Test_canUnInstall(t *testing.T) {

	t.Run("should uninstall when istio is installed", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.9.2",
			TargetVersion:    "",
			PilotVersion:     "",
			DataPlaneVersion: "1.9.2",
		}

		// when
		got := canUninstall(istioVersion)

		// then
		require.True(t, got)
	})

	t.Run("should not uninstall when istio is not installed", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			TargetVersion:    "",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := canUninstall(istioVersion)

		// then
		require.False(t, got)
	})

	t.Run("should not uninstall when istio ctl is not installed", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "",
			TargetVersion:    "1.11.2",
			PilotVersion:     "1.11.2",
			DataPlaneVersion: "1.11.2",
		}

		// when
		got := canUninstall(istioVersion)

		// then
		require.False(t, got)
	})
	t.Run("should not matter to uninstall if client version and data plane diverge", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.9.0",
			TargetVersion:    "1.20.2",
			PilotVersion:     "1.11.2",
			DataPlaneVersion: "1.11.2",
		}

		// when
		got := canUninstall(istioVersion)

		// then
		require.True(t, got)
	})
}

func Test_canInstall(t *testing.T) {
	t.Run("should install when client and pilot versions are empty", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.9.2",
			TargetVersion:    "",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := canInstall(istioVersion)

		// then
		require.True(t, got)
	})

	t.Run("should update when client and pilot versions values are not empty", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			TargetVersion:    "",
			PilotVersion:     "1.11.1",
			DataPlaneVersion: "1.11.1",
		}

		// when
		got := canInstall(istioVersion)

		// then
		require.False(t, got)
	})
}

func Test_canUpdate(t *testing.T) {
	t.Run("should not allow update when client version is more than one minor behind the target version", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.0.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.0.0",
			DataPlaneVersion: "1.0.0",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.False(t, result)
	})

	t.Run("should allow update when permissible downgrade scenario is detected for pilot", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.1.0",
			PilotVersion:     "1.2.0",
			DataPlaneVersion: "1.1.0",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.True(t, result)
	})

	t.Run("should not allow update when downgrade scenario is detected for pilot", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.1.0",
			PilotVersion:     "1.3.0",
			DataPlaneVersion: "1.1.0",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.False(t, result)
	})

	t.Run("should allow update when permissible downgrade scenario is detected for data plane", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.1.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.5",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.True(t, result)
	})

	t.Run("should not allow update when downgrade scenario is detected for data plane", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.1.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.3.0",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.False(t, result)
	})

	t.Run("should not allow update when more than one minor upgrade is detected for pilot", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.0.0",
			DataPlaneVersion: "1.1.0",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.False(t, result)
	})

	t.Run("should not allow update when more than one minor upgrade is detected for data plane", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.0.0",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.False(t, result)
	})

	t.Run("should allow update when less than one minor upgrade is detected for pilot and data plane ", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.True(t, result)
	})

	t.Run("should allow update when all versions match", func(t *testing.T) {
		// given
		version := actions.IstioStatus{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.2.0",
			DataPlaneVersion: "1.2.0",
		}

		// when
		result, _ := canUpdate(version)

		// then
		require.True(t, result)
	})
}

func Test_isMismatchPresent(t *testing.T) {
	t.Run("should return false if version string is semver incompatible", func(t *testing.T) {
		// given
		differentVersions := actions.IstioStatus{
			ClientVersion:    "version1",
			PilotVersion:     "version2",
			DataPlaneVersion: "version3",
		}

		// when
		got := isMismatchPresent(differentVersions)

		// then
		require.False(t, got)
	})
	t.Run("Different Pilot and DataPlane versions is a mismatch", func(t *testing.T) {
		// given
		differentVersions := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			PilotVersion:     "1.11.1",
			DataPlaneVersion: "1.11.2",
		}

		// when
		got := isMismatchPresent(differentVersions)

		// then
		require.True(t, got)
	})

	t.Run("Same Pilot and DataPlane versions is not a mismatch", func(t *testing.T) {
		// given
		sameVersions := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			PilotVersion:     "1.11.2",
			DataPlaneVersion: "1.11.2",
		}

		// when
		got := isMismatchPresent(sameVersions)

		// then
		require.False(t, got)
	})
}

func Test_isClientCompatible(t *testing.T) {
	t.Run("should return false if version string is semver incompatible", func(t *testing.T) {
		// given
		badVersions := actions.IstioStatus{
			ClientVersion:    "version1",
			PilotVersion:     "version2",
			DataPlaneVersion: "version3",
		}

		// when
		got := isClientCompatibleWithTargetVersion(badVersions)

		// then
		require.False(t, got)
	})

	t.Run("should return true when client and target versions are the same", func(t *testing.T) {
		// given
		exactSameClientVersion := actions.IstioStatus{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.1.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := isClientCompatibleWithTargetVersion(exactSameClientVersion)

		// then
		require.True(t, got)
	})

	t.Run("should return true when client and target versions are of the same minor and different patch and client version is higher than target", func(t *testing.T) {
		// given
		sameMinorClientVersion := actions.IstioStatus{
			ClientVersion:    "1.1.1",
			TargetVersion:    "1.1.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := isClientCompatibleWithTargetVersion(sameMinorClientVersion)

		// then
		require.True(t, got)
	})

	t.Run("should return true when client and target versions are of the same minor and different patch and target version is higher than client", func(t *testing.T) {
		// given
		sameMinorClientVersion := actions.IstioStatus{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.1.1",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := isClientCompatibleWithTargetVersion(sameMinorClientVersion)

		// then
		require.True(t, got)
	})

	t.Run("should return true when client and target versions are among one minor and of the same patch and client version is higher than target", func(t *testing.T) {
		// given
		oneHigherMinorClientVersion := actions.IstioStatus{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.1.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := isClientCompatibleWithTargetVersion(oneHigherMinorClientVersion)

		// then
		require.True(t, got)
	})

	t.Run("should return true when client and target versions are among one minor and of the same patch and target version is higher than client", func(t *testing.T) {
		// given
		oneLowerMinorClientVersion := actions.IstioStatus{
			ClientVersion:    "1.1.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := isClientCompatibleWithTargetVersion(oneLowerMinorClientVersion)

		// then
		require.True(t, got)
	})

	t.Run("should return false when client and target versions are not among one minor and target version is higher than client", func(t *testing.T) {
		// given
		twoLowerMinorClientVersion := actions.IstioStatus{
			ClientVersion:    "1.0.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := isClientCompatibleWithTargetVersion(twoLowerMinorClientVersion)

		// then
		require.False(t, got)
	})

	t.Run("should return false when client and target versions are not among one minor and client version is higher than target", func(t *testing.T) {
		// given
		greaterThanOneMinorClientVersion := actions.IstioStatus{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.0.0",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		// when
		got := isClientCompatibleWithTargetVersion(greaterThanOneMinorClientVersion)

		// then
		require.False(t, got)
	})
}

func Test_isComponentCompatible(t *testing.T) {
	componentName := "component"

	t.Run("should return false when version string is semver incompatible", func(t *testing.T) {
		// given
		badVersions := actions.IstioStatus{
			ClientVersion:    "version1",
			PilotVersion:     "version2",
			DataPlaneVersion: "version3",
		}

		// when
		got, err := isComponentCompatible(badVersions.PilotVersion, badVersions.TargetVersion, componentName)

		// then
		require.Error(t, err)
		require.False(t, got)
	})

	t.Run("should return true when pilot and target versions are equal", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.2.3",
			TargetVersion:    "1.2.3",
			PilotVersion:     "1.2.3",
			DataPlaneVersion: "1.2.3",
		}

		// when
		got, err := isComponentCompatible(istioVersion.PilotVersion, istioVersion.TargetVersion, componentName)

		// then
		require.NoError(t, err)
		require.True(t, got)
	})

	t.Run("should return true when pilot and targets version are vary only in patch", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.2.3",
			TargetVersion:    "1.2.3",
			PilotVersion:     "1.2.0",
			DataPlaneVersion: "1.2.3",
		}

		// when
		got, err := isComponentCompatible(istioVersion.PilotVersion, istioVersion.TargetVersion, componentName)

		// then
		require.NoError(t, err)
		require.True(t, got)
	})

	t.Run("should return true when pilot version is one minor lower than target", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.2.3",
			TargetVersion:    "1.2.3",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.2.3",
		}

		// when
		got, err := isComponentCompatible(istioVersion.PilotVersion, istioVersion.TargetVersion, componentName)

		// then
		require.NoError(t, err)
		require.True(t, got)
	})

	t.Run("should return true when pilot version is one minor higher than target", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.2.3",
			TargetVersion:    "1.2.3",
			PilotVersion:     "1.3.0",
			DataPlaneVersion: "1.2.3",
		}

		// when
		got, err := isComponentCompatible(istioVersion.PilotVersion, istioVersion.TargetVersion, componentName)

		// then
		require.NoError(t, err)
		require.True(t, got)
	})

	t.Run("should return false when pilot version is more than one minor lower than target", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.2.3",
			TargetVersion:    "1.2.3",
			PilotVersion:     "1.0.0",
			DataPlaneVersion: "1.2.3",
		}

		// when
		got, err := isComponentCompatible(istioVersion.PilotVersion, istioVersion.TargetVersion, componentName)

		// then
		require.Error(t, err)
		require.False(t, got)
	})

	t.Run("should return false when pilot version is more than one minor higher than target", func(t *testing.T) {
		// given
		istioVersion := actions.IstioStatus{
			ClientVersion:    "1.2.3",
			TargetVersion:    "1.2.3",
			PilotVersion:     "1.4.0",
			DataPlaneVersion: "1.2.3",
		}

		// when
		got, err := isComponentCompatible(istioVersion.PilotVersion, istioVersion.TargetVersion, componentName)

		// then
		require.Error(t, err)
		require.False(t, got)
	})
}

func Test_amongOneMinor(t *testing.T) {
	t.Run("Downgrade of PilotVersion with same minor version is permitted", func(t *testing.T) {
		// given
		sameMinorPilotVersion := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			TargetVersion:    "1.11.2",
			PilotVersion:     "1.11.6",
			DataPlaneVersion: "1.11.2",
		}
		pilotHelperVersion, err := newHelperVersionFrom(sameMinorPilotVersion.PilotVersion)
		require.NoError(t, err)
		targetHelperVersion, err := newHelperVersionFrom(sameMinorPilotVersion.TargetVersion)
		require.NoError(t, err)

		// when
		got := amongOneMinor(pilotHelperVersion, targetHelperVersion)

		// then
		require.True(t, got)
	})

	t.Run("Upgrade of PilotVersion with same minor version is permitted", func(t *testing.T) {
		// given
		sameMinorPilotVersion := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			TargetVersion:    "1.11.2",
			PilotVersion:     "1.11.1",
			DataPlaneVersion: "1.11.2",
		}
		pilotHelperVersion, err := newHelperVersionFrom(sameMinorPilotVersion.PilotVersion)
		require.NoError(t, err)
		targetHelperVersion, err := newHelperVersionFrom(sameMinorPilotVersion.TargetVersion)
		require.NoError(t, err)

		// when
		got := amongOneMinor(pilotHelperVersion, targetHelperVersion)

		// then
		require.True(t, got)
	})

	t.Run("Downgrade of PilotVersion with one minor version is permitted", func(t *testing.T) {
		// given
		oneMinorPilotVersion := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			TargetVersion:    "1.11.2",
			PilotVersion:     "1.12.6",
			DataPlaneVersion: "1.11.2",
		}
		pilotHelperVersion, err := newHelperVersionFrom(oneMinorPilotVersion.PilotVersion)
		require.NoError(t, err)
		targetHelperVersion, err := newHelperVersionFrom(oneMinorPilotVersion.TargetVersion)
		require.NoError(t, err)

		// when
		got := amongOneMinor(pilotHelperVersion, targetHelperVersion)

		// then
		require.True(t, got)
	})

	t.Run("Upgrade of PilotVersion with one minor version is permitted", func(t *testing.T) {
		// given
		oneMinorPilotVersion := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			TargetVersion:    "1.11.2",
			PilotVersion:     "1.10.1",
			DataPlaneVersion: "1.11.2",
		}
		pilotHelperVersion, err := newHelperVersionFrom(oneMinorPilotVersion.PilotVersion)
		require.NoError(t, err)
		targetHelperVersion, err := newHelperVersionFrom(oneMinorPilotVersion.TargetVersion)
		require.NoError(t, err)

		// when
		got := amongOneMinor(pilotHelperVersion, targetHelperVersion)

		// then
		require.True(t, got)
	})

	t.Run("Downgrade of PilotVersion with more than one minor version is NOT permitted", func(t *testing.T) {
		// given
		greaterThanOneMinorPilotVersion := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			TargetVersion:    "1.11.2",
			PilotVersion:     "1.13.6",
			DataPlaneVersion: "1.11.2",
		}
		pilotHelperVersion, err := newHelperVersionFrom(greaterThanOneMinorPilotVersion.PilotVersion)
		require.NoError(t, err)
		targetHelperVersion, err := newHelperVersionFrom(greaterThanOneMinorPilotVersion.TargetVersion)
		require.NoError(t, err)

		// when
		got := amongOneMinor(pilotHelperVersion, targetHelperVersion)

		// then
		require.False(t, got)
	})

	t.Run("Upgrade of PilotVersion with more than one minor version is NOT permitted", func(t *testing.T) {
		// given
		lesserThanOneMinorPilotVersion := actions.IstioStatus{
			ClientVersion:    "1.11.2",
			TargetVersion:    "1.11.2",
			PilotVersion:     "1.9.1",
			DataPlaneVersion: "1.11.2",
		}
		pilotHelperVersion, err := newHelperVersionFrom(lesserThanOneMinorPilotVersion.PilotVersion)
		require.NoError(t, err)
		targetHelperVersion, err := newHelperVersionFrom(lesserThanOneMinorPilotVersion.TargetVersion)
		require.NoError(t, err)

		// when
		got := amongOneMinor(pilotHelperVersion, targetHelperVersion)

		// then
		require.False(t, got)
	})
}

func Test_generateNewManifestWithoutIstioOperatorFrom(t *testing.T) {

	t.Run("should generate empty manifest from empty input manifest", func(t *testing.T) {
		// when
		result, err := manifest.GenerateNewManifestWithoutIstioOperatorFrom("")

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
		result, err := manifest.GenerateNewManifestWithoutIstioOperatorFrom(istioManifestWithoutIstioOperator)

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
		result, err := manifest.GenerateNewManifestWithoutIstioOperatorFrom(istioManifest)

		// then
		require.NoError(t, err)
		require.Contains(t, result, "Kind1")
		require.Contains(t, result, "Kind2")
		require.NotContains(t, result, "IstioOperator")
	})

}
