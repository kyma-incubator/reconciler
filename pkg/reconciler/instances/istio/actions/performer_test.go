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
	istioctlMockVersion = `{
	"clientVersion": {
	  "version": "1.11.1",
	},
	"meshVersion": [
	  {
		"Component": "pilot",
		"Info": {
		  "version": "1.11.1",
		}
	  }
	],
	"dataPlaneVersion": [
	  {
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

	t.Run("should not install when istioctl binary could not be found in env", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, "", log, &cmder)

		/// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istioctl binary could not be found")
		cmder.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	})

	t.Run("should not install when istio operator could not be found in manifest", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, "", log, &cmder)

		/// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
		cmder.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	})

	t.Run("should not install Istio when istioctl returned an error", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, istioManifest, log, &cmder)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	})

	t.Run("should install Istio when istioctl command was successful", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, istioManifest, log, &cmder)

		// then
		require.NoError(t, err)
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
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

	t.Run("should get istioctl version", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		cmder := istioctlmocks.Commander{}
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return([]byte(""), nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		ver, err := wrapper.Version(kubeConfig, log, &cmder)

		// then
		require.EqualValues(t, ver, IstioVersion{clientVersion: "", pilotVersion: "", dataPlaneVersion: ""})
		require.NoError(t, err)
		cmder.AssertCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	})
}
