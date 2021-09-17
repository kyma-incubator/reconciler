package actions

import (
	"os"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	istioctlmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

const (
	istioManifest = `
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
	istioctlMockSimpleVersion = `no running Istio pods in "istio-system"
{
	"clientVersion": {
	"version": "1.11.2",
	"revision": "96710172e1e47cee227e7e8dd591a318fdfe0326",
	"golang_version": "go1.16.7",
	"status": "Clean",
	"tag": "1.11.2"
	}
}`

	istioctlMockCompleteVersion = `{
		"clientVersion": {
		  "version": "1.11.1",
		  "revision": "ce6205d503e5c5e41af496ebbe01ece7dc6c3547",
		  "golang_version": "go1.16.7",
		  "status": "Clean",
		  "tag": "1.11.1"
		},
		"meshVersion": [
		  {
			"Component": "pilot",
			"Info": {
			  "version": "1.11.1",
			  "revision": "ce6205d503e5c5e41af496ebbe01ece7dc6c3547",
			  "golang_version": "",
			  "status": "Clean",
			  "tag": "1.11.1"
			}
		  }
		],
		"dataPlaneVersion": [
		  {
			"ID": "istio-ingressgateway-59ccd8f5-cpwxx.istio-system",
			"IstioVersion": "1.11.1"
		  }
		]
	  }`
)

func Test_DefaultIstioPerformer_Install(t *testing.T) {

	err := os.Setenv("ISTIOCTL_PATH", "path")
	require.NoError(t, err)
	kubeConfig := "kubeConfig"
	manifest := "manifest"
	kubeClient := mocks.Client{}
	cmder := istioctlmocks.Commander{}
	log, err := logger.NewLogger(false)
	require.NoError(t, err)

	t.Run("should not install when istio operator could not be found in manifest", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, "", log, &cmder)

		/// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
		cmder.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should not install Istio when istioctl returned an error", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, istioManifest, log, &cmder)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should install Istio when istioctl command was successful", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, istioManifest, log, &cmder)

		// then
		require.NoError(t, err)
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

}

func Test_DefaultIstioPerformer_PatchMutatingWebhook(t *testing.T) {

	err := os.Setenv("ISTIOCTL_PATH", "path")
	require.NoError(t, err)
	log := logger.NewLogger(false)

	t.Run("should not patch MutatingWebhookConfiguration when kubeclient had returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		wrapper := NewDefaultIstioPerformer(&cmder)
		kubeClient := mocks.Client{}
		kubeClient.On("PatchUsingStrategy", "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", mock.Anything, types.JSONPatchType).Return(errors.New("kubeclient error"))

		// when
		err = wrapper.PatchMutatingWebhook(&kubeClient, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "kubeclient error")
	})

	t.Run("should patch MutatingWebhookConfiguration when kubeclient had not returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		wrapper := NewDefaultIstioPerformer(&cmder)
		kubeClient := mocks.Client{}
		kubeClient.On("PatchUsingStrategy", "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", mock.Anything, types.JSONPatchType).Return(nil)

		// when
		err = wrapper.PatchMutatingWebhook(&kubeClient, log)

		// then
		require.NoError(t, err)
	})

}

func Test_extractIstioOperatorContextFrom(t *testing.T) {

	t.Run("should not extract istio operator from manifest that does not contain istio operator", func(t *testing.T) {
		// when
		result, err := extractIstioOperatorContextFrom("")

		// then
		require.Empty(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not be found")
	})

	t.Run("should extract istio operator from combo manifest", func(t *testing.T) {
		// when
		result, err := extractIstioOperatorContextFrom(istioManifest)

		// then
		require.NoError(t, err)
		require.Contains(t, result, "IstioOperator")
	})

}

func Test_DefaultIstioPerformer_Version(t *testing.T) {
	kubeConfig := "kubeConfig"
	log, err := logger.NewLogger(false)
	require.NoError(t, err)

	t.Run("should not proceed if the version command returns an empty string", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		cmder := istioctlmocks.Commander{}
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(""), nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		ver, err := wrapper.Version(kubeConfig, log, &cmder)

		// then
		require.Empty(t, ver)
		require.Error(t, err)
		require.Contains(t, err.Error(), "command is empty")
	})

	t.Run("should get only the client version when istio is not yet installed on the cluster", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		cmder := istioctlmocks.Commander{}
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(istioctlMockSimpleVersion), nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		ver, err := wrapper.Version(kubeConfig, log, &cmder)

		// then
		require.EqualValues(t, ver, IstioVersion{ClientVersion: "1.11.2"})
		require.NoError(t, err)
		cmder.AssertCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		cmder.AssertNumberOfCalls(t, "Version", 1)
	})

	t.Run("should get all the expected versions when istio installed on the cluster", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		cmder := istioctlmocks.Commander{}
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(istioctlMockCompleteVersion), nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		ver, err := wrapper.Version(kubeConfig, log, &cmder)

		// then
		require.EqualValues(t, ver, IstioVersion{ClientVersion: "1.11.1", PilotVersion: "1.11.1", DataPlaneVersion: "1.11.1"})
		require.NoError(t, err)
		cmder.AssertCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		cmder.AssertNumberOfCalls(t, "Version", 1)
	})
}
