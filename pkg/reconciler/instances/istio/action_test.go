package istio

import (
	"context"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
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

func Test_ReconcileAction_Run(t *testing.T) {

	t.Run("should not perform istio reconcile action", func(t *testing.T) {
		// TODO: uncomment when chartprovider mock is resolved
		//// given
		//performer := istiomocks.IstioPerformer{}
		//commander := istioctlmocks.Commander{}
		//action := ReconcileAction{performer: &performer, commander: &commander}
		//
		//// when
		//err := action.Run("0.0.0", "profile", nil, newFakeServiceContext(t))
		//
		//// then
		//require.Error(t, err) // it should not return error, refactor
	})

}

func newFakeServiceContext(t *testing.T) *service.ActionContext {
	mockClient := &k8smocks.Client{}
	mockClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
	// We create './test_files/0.0.0/success.yaml' to trick the
	// WorkspaceFactory into thinking that we don't need to
	// clone the kyma repo. This is a temporary workaround
	// since we can't currently mock WorkspaceFactory.
	fakeFactory, err := workspace.NewFactory("./test_files", log.NewOptionalLogger(true))
	require.NoError(t, err)
	logger := log.NewOptionalLogger(true)
	chartProvider, err := chart.NewProvider(fakeFactory, logger)
	require.NoError(t, err)

	return &service.ActionContext{
		KubeClient:       mockClient,
		Context:          context.Background(),
		WorkspaceFactory: fakeFactory,
		Logger:           logger,
		ChartProvider:    chartProvider,
	}
}
