package istio

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	actionsmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions/mocks"
	istioctlmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"testing"

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

	t.Run("should not perform any istio action when provider returns an error ", func(t *testing.T) {
		// given
		fakeFactory := workspacemocks.Factory{}
		fakeProvider := chartmocks.Provider{}
		fakeProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(nil, errors.New("Provider error"))
		actionContext := newFakeServiceContext(&fakeFactory, &fakeProvider)
		performer := actionsmocks.IstioPerformer{}
		commander := istioctlmocks.Commander{}
		action := ReconcileAction{performer: &performer, commander: &commander}

		// when
		err := action.Run("version", "profile", nil, actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, "Provider error", err.Error())
	})

	t.Run("should not perform any istio action when provider returns an error ", func(t *testing.T) {
		// given
		fakeFactory := workspacemocks.Factory{}
		fakeProvider := chartmocks.Provider{}
		fakeProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).Return(chart.Manifest{}, nil)
		actionContext := newFakeServiceContext(&fakeFactory, &fakeProvider)
		performer := actionsmocks.IstioPerformer{}
		commander := istioctlmocks.Commander{}
		action := ReconcileAction{performer: &performer, commander: &commander}

		// when
		err := action.Run("version", "profile", nil, actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, "Provider error", err.Error())
	})

}

func newFakeServiceContext(factory workspace.Factory, provider chart.Provider) *service.ActionContext {
	mockClient := &k8smocks.Client{}
	mockClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
	logger := log.NewOptionalLogger(true)

	return &service.ActionContext{
		KubeClient:       mockClient,
		Context:          context.Background(),
		WorkspaceFactory: factory,
		Logger:           logger,
		ChartProvider:    provider,
	}
}

func TestGetAppVersionFromChart(t *testing.T) {

}

func TestShouldInstall(t *testing.T) {
	t.Run("If client version and pilot version values are empty, we install", func(t *testing.T) {
		//given
		randomVersion := actions.IstioVersion{
			ClientVersion:    "1.9.2",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}

		//when
		got := shouldInstall(randomVersion)

		//then
		require.Equal(t, true, got)
	})

	t.Run("If client version and pilot version values are not empty, we update", func(t *testing.T) {
		//given
		randomVersion := actions.IstioVersion{
			ClientVersion:    "1.11.2",
			PilotVersion:     "1.11.1",
			DataPlaneVersion: "1.11.1",
		}

		//when
		got := shouldInstall(randomVersion)

		//then
		require.Equal(t, false, got)
	})
}

func TestIsClientVersionAcceptable(t *testing.T) {
	t.Run("Istioctl version and appVersion are not the same", func(t *testing.T) {
		//given
		randomVersion := actions.IstioVersion{
			ClientVersion:    "1.9.2",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}
		appVersion := "1.11.2"

		//when
		got := canUpdate(randomVersion, appVersion)

		//then
		require.Equal(t, false, got)
	})

	t.Run("Istioctl version and appVersion are the same", func(t *testing.T) {
		//given
		randomVersion := actions.IstioVersion{
			ClientVersion:    "1.11.2",
			PilotVersion:     "",
			DataPlaneVersion: "",
		}
		appVersion := "1.11.2"

		//when
		got := canUpdate(randomVersion, appVersion)

		//then
		require.Equal(t, true, got)
	})
}

func TestCanUpdate(t *testing.T) {
	t.Run("If the minor version difference is 2 or more, we cannot update", func(t *testing.T) {
		//given
		oneNineVersions := actions.IstioVersion{
			ClientVersion:    "1.9.2",
			PilotVersion:     "1.9.2",
			DataPlaneVersion: "1.9.2",
		}
		oneEightVersions := actions.IstioVersion{
			ClientVersion:    "1.8.6",
			PilotVersion:     "1.8.6",
			DataPlaneVersion: "1.8.6",
		}
		appVersion := "1.11.2"

		//when
		got1 := canUpdate(oneNineVersions, appVersion)
		got2 := canUpdate(oneEightVersions, appVersion)

		//then
		require.Equal(t, false, got1)
		require.Equal(t, false, got2)
	})

	t.Run("If the minor version difference is less than or equal to 1, we can update", func(t *testing.T) {
		//given
		oneTenVersions := actions.IstioVersion{
			ClientVersion:    "1.10.2",
			PilotVersion:     "1.10.2",
			DataPlaneVersion: "1.10.2",
		}
		oneElevenVersions := actions.IstioVersion{
			ClientVersion:    "1.11.1",
			PilotVersion:     "1.11.1",
			DataPlaneVersion: "1.11.1",
		}
		appVersion := "1.11.2"

		//when
		got1 := canUpdate(oneTenVersions, appVersion)
		got2 := canUpdate(oneElevenVersions, appVersion)

		//then
		require.Equal(t, true, got1)
		require.Equal(t, true, got2)
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

func TestIsDowngrade(t *testing.T) {

	t.Run("Lower app version compared to istio version is a downgrade", func(t *testing.T) {
		//given
		istioVersion := actions.IstioVersion{
			ClientVersion:    "1.11.2",
			PilotVersion:     "1.11.2",
			DataPlaneVersion: "1.11.2",
		}
		appVersion := "1.11.1"

		//when
		got := isDowngrade(istioVersion, appVersion)

		//then
		require.Equal(t, true, got)
	})

	t.Run("Similar app and istio versions is not a downgrade", func(t *testing.T) {
		//given
		istioVersion := actions.IstioVersion{
			ClientVersion:    "1.11.2",
			PilotVersion:     "1.11.2",
			DataPlaneVersion: "1.11.2",
		}
		appVersion := "1.11.2"

		//when
		got := isDowngrade(istioVersion, appVersion)

		//then
		require.Equal(t, false, got)
	})

	t.Run("Higher app version compared to istio version is a downgrade", func(t *testing.T) {
		//given
		istioVersion := actions.IstioVersion{
			ClientVersion:    "1.11.2",
			PilotVersion:     "1.11.2",
			DataPlaneVersion: "1.11.2",
		}
		appVersion := "1.11.3"

		//when
		got := isDowngrade(istioVersion, appVersion)

		//then
		require.Equal(t, false, got)
	})
}
