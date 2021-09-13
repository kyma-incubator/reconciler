package actions

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	istioctlmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
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
)

func Test_NewDefaultIstioPerformer(t *testing.T) {

	kubeConfig := "kubeConfig"
	manifest := "manifest"
	kubeClient := mocks.Client{}
	log := logger.NewLogger(false)
	cmder := istioctlmocks.Commander{}

	t.Run("should not create wrapper when istioctl binary could not be found in env", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "")
		require.NoError(t, err)

		// when
		wrapper, err := NewDefaultIstioPerformer(kubeConfig, manifest, &kubeClient, log, &cmder)

		/// then
		require.Nil(t, wrapper)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istioctl binary could not be found")
	})

	t.Run("should not create wrapper when istioctl operator could not be found in manifest", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)

		// when
		wrapper, err := NewDefaultIstioPerformer(kubeConfig, manifest, &kubeClient, log, &cmder)

		/// then
		require.Nil(t, wrapper)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
	})

	t.Run("should create wrapper when all required properties are present", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)

		// when
		wrapper, err := NewDefaultIstioPerformer(kubeConfig, istioManifest, &kubeClient, log, &cmder)

		/// then
		require.NotNil(t, wrapper)
		require.NoError(t, err)
	})

}

func Test_DefaultIstioPerformer_Install(t *testing.T) {

	err := os.Setenv("ISTIOCTL_PATH", "path")
	require.NoError(t, err)
	kubeConfig := "kubeConfig"
	kubeClient := mocks.Client{}
	log := logger.NewLogger(false)

	t.Run("should not install Istio when istioctl returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("istioctl error"))
		wrapper, err := NewDefaultIstioPerformer(kubeConfig, istioManifest, &kubeClient, log, &cmder)
		require.NoError(t, err)

		// when
		err = wrapper.Install(kubeConfig, istioManifest, log, &cmder)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
	})

	t.Run("should install Istio when istioctl command was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
		wrapper, err := NewDefaultIstioPerformer(kubeConfig, istioManifest, &kubeClient, log, &cmder)
		require.NoError(t, err)

		// when
		err = wrapper.Install(kubeConfig, istioManifest, log, &cmder)

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
