package ory

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"gopkg.in/yaml.v2"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/db"
	hydramocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/hydra/mocks"
	oryk8smock "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/k8s/mocks"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	memoryYaml = `
    global:
      ory:
        hydra:
          persistence:
            enabled: false
    hydra:
      enabled: true`

	postgresqlYaml = `
    global:
      ory:
        hydra:
          persistence:
            enabled: true
            postgresql:
              enabled: true
    hydra: 
      enabled: true`

	hydraDisabledYaml = `
    global:
      ory:
        hydra:
          persistence:
            enabled: false
    hydra: 
      enabled: false`

	hydraNotExistentYaml = `
    global:
      ory:
        hydra:
          persistence:
            enabled: false
    hydra: 
      enabled: false`
)

const (
	profileName   = "profile"
	componentName = "test-ory"
	inMemoryURL   = "sqlite://file::memory:?cache=shared&busy_timeout=5000&_fk=true"
)

var chartDir = filepath.Join("test", "resources")

func Test_PostReconcile_Run(t *testing.T) {
	t.Parallel()
	t.Run("should call hydra sync when we are inMemory mode", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		hydraClient := hydramocks.Syncer{}
		hydraClient.On("TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil)
		rolloutMock := oryk8smock.RolloutHandler{}
		values, err := unmarshalTestValues(memoryYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := postReconcileAction{&oryAction{step: "post-reconcile"}, &hydraClient, &rolloutMock}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		hydraClient.AssertCalled(t, "TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	})
	t.Run("should not call hydra sync when we are in persistence enabled mode", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		hydraClient := hydramocks.Syncer{}
		rolloutMock := oryk8smock.RolloutHandler{}
		values, err := unmarshalTestValues(postgresqlYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := postReconcileAction{&oryAction{step: "post-reconcile"}, &hydraClient, &rolloutMock}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		hydraClient.AssertNotCalled(t, "TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	})

	t.Run("should return error when synchronization failed", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		hydraClient := hydramocks.Syncer{}
		hydraClient.On("TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("Failed to trigger hydra Synchronization"))
		rolloutMock := oryk8smock.RolloutHandler{}
		values, err := unmarshalTestValues(memoryYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := postReconcileAction{&oryAction{step: "post-reconcile"}, &hydraClient, &rolloutMock}

		// when
		err = action.Run(actionContext)

		// Then
		require.Error(t, err, "Failed to trigger hydra Synchronization")
		hydraClient.AssertCalled(t, "TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	})
	t.Run("should return error when read of action context failed", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		hydraClient := hydramocks.Syncer{}
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(nil,
			errors.New("Failed to read configuration"))
		rolloutMock := oryk8smock.RolloutHandler{}
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := postReconcileAction{&oryAction{step: "post-reconcile"}, &hydraClient, &rolloutMock}

		// when
		err := action.Run(actionContext)

		// Then
		require.Error(t, err, "Failed to read configuration")
		hydraClient.AssertNotCalled(t, "TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	})

	t.Run("should not skip Hydra resources, when Hydra is disabled", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		hydraClient := hydramocks.Syncer{}
		hydraClient.On("TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("Unexpected invocation"))
		rolloutMock := oryk8smock.RolloutHandler{}
		rolloutMock.On("rolloutHydraDeployment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("Unexpected invocation"))
		values, err := unmarshalTestValues(hydraDisabledYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := postReconcileAction{&oryAction{step: "post-reconcile"}, &hydraClient, &rolloutMock}

		// when
		err = action.Run(actionContext)

		// Then
		require.NoError(t, err)
		hydraClient.AssertNotCalled(t, "TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		hydraClient.AssertNotCalled(t, "rolloutHydraDeployment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("should not continue with action, when Hydra is not configured in yaml", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		hydraClient := hydramocks.Syncer{}
		hydraClient.On("TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("Failed to trigger hydra Synchronization"))
		rolloutMock := oryk8smock.RolloutHandler{}
		rolloutMock.On("rolloutHydraDeployment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("Unexpected invocation"))
		values, err := unmarshalTestValues(hydraNotExistentYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := postReconcileAction{&oryAction{step: "post-reconcile"}, &hydraClient, &rolloutMock}

		// when
		err = action.Run(actionContext)

		// Then
		require.NoError(t, err)
		hydraClient.AssertNotCalled(t, "TriggerSynchronization", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		hydraClient.AssertNotCalled(t, "rolloutHydraDeployment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func Test_PreInstallAction_Run(t *testing.T) {
	t.Parallel()
	t.Run("should not perform any action when chart configuration returned an error", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(nil,
			errors.New("Configuration error"))
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Failed to retrieve ory chart values")
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertNotCalled(t, "Clientset")
	})

	t.Run("should create Oathkeeper jwks, but not create database secret when hydra is disabled", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		values, err := unmarshalTestValues(hydraDisabledYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)

		secret, err := clientSet.CoreV1().Secrets(jwksNamespacedName.Namespace).Get(actionContext.Context, jwksNamespacedName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, jwksNamespacedName.Name, secret.Name)
		require.Equal(t, jwksNamespacedName.Namespace, secret.Namespace)
		require.NotEmpty(t, secret.Data)

		secret, err = clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.Error(t, err)
		require.True(t, kerrors.IsNotFound(err))
	})

	t.Run("should create Oathkeeper jwks, but not create database secret when hydra is not in config", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		values, err := unmarshalTestValues(hydraNotExistentYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)

		secret, err := clientSet.CoreV1().Secrets(jwksNamespacedName.Namespace).Get(actionContext.Context, jwksNamespacedName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, jwksNamespacedName.Name, secret.Name)
		require.Equal(t, jwksNamespacedName.Namespace, secret.Namespace)
		require.NotEmpty(t, secret.Data)

		secret, err = clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.Error(t, err)
		require.True(t, kerrors.IsNotFound(err))
	})

	t.Run("should not perform any action when kubernetes clientset returned an error", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		values, err := unmarshalTestValues(memoryYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		kubeClient := k8smocks.Client{}
		kubeClient.On("Clientset").Return(nil, errors.New("cannot get secret"))
		actionContext := newFakeServiceContext(&factory, &provider, &kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err = action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot get secret")
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
	})

	t.Run("should create jwks secret when secret does not exist", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		emptyMap := make(map[string]interface{})
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(emptyMap, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
		secret, err := clientSet.CoreV1().Secrets(jwksNamespacedName.Namespace).Get(actionContext.Context, jwksNamespacedName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, jwksNamespacedName.Name, secret.Name)
		require.Equal(t, jwksNamespacedName.Namespace, secret.Namespace)
		require.NotEmpty(t, secret.Data)
	})

	t.Run("should not create jwks secret when secret exists", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		emptyMap := make(map[string]interface{})
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(emptyMap, nil)
		existingJwksSecret := fixSecretJwks()
		clientSet := fake.NewSimpleClientset(existingJwksSecret)
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
		secret, err := clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, jwksNamespacedName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, jwksNamespacedName.Name, secret.Name)
		require.Equal(t, oryNamespace, secret.Namespace)
		require.Equal(t, getJWKSData(), secret.Data)
	})

	t.Run("should create ory secret when secret does not exist", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		values, err := unmarshalTestValues(memoryYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
		secret, err := clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, databaseSecretName, secret.Name)
		require.Equal(t, oryNamespace, secret.Namespace)
		require.Equal(t, inMemoryURL, secret.StringData["dsn"])
	})

	t.Run("should not update ory secret when secret exist and has a valid data", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		values, err := unmarshalTestValues(memoryYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		existingSecret := fixSecretMemory()
		clientSet := fake.NewSimpleClientset(existingSecret)
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
		secret, err := clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, databaseSecretName, secret.Name)
		require.Equal(t, oryNamespace, secret.Namespace)
		require.Equal(t, "", secret.StringData["dsn"])
		require.Equal(t, []byte(inMemoryURL), secret.Data["dsn"])
	})

	t.Run("should update ory secret when secret exist and has an outdated values", func(t *testing.T) {
		// given
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		rolloutMock := oryk8smock.RolloutHandler{}
		rolloutMock.On("RolloutAndWaitForDeployment", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		values, err := unmarshalTestValues(postgresqlYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		existingSecret := fixSecretMemory()
		hydraDeployment := fixOryHydraDeployment()
		clientSet := fake.NewSimpleClientset(existingSecret, hydraDeployment)
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
		secret, err := clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, databaseSecretName, secret.Name)
		require.Equal(t, oryNamespace, secret.Namespace)
		require.Contains(t, secret.StringData["dsn"], "postgres")
		require.NotContains(t, secret.StringData["dsn"], inMemoryURL)
	})
}

func Test_PostDeleteAction_Run(t *testing.T) {
	t.Run("should not perform any action when kubernetes clientset returned an error", func(t *testing.T) {
		// given
		oryFinalizersMock := oryk8smock.OryFinalizersHandler{}
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		emptyMap := make(map[string]interface{})
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(emptyMap, nil)
		kubeClient := k8smocks.Client{}
		kubeClient.On("Clientset").Return(nil, errors.New("failed to retrieve native Kubernetes GO client"))
		actionContext := newFakeServiceContext(&factory, &provider, &kubeClient)
		action := postDeleteAction{&oryAction{step: "post-delete"}, &oryFinalizersMock}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to retrieve native Kubernetes GO client")
		kubeClient.AssertCalled(t, "Clientset")
		kubeClient.AssertNotCalled(t, "Kubeconfig")
	})

	t.Run("should not perform any action when DB secret does not exist", func(t *testing.T) {
		// given
		oryFinalizersMock := oryk8smock.OryFinalizersHandler{}
		oryFinalizersMock.On("FindAndDeleteOryFinalizers",
			mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(errors.New("FindAndDeleteOryFinalizers error"))
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		emptyMap := make(map[string]interface{})
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(emptyMap, nil)
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := postDeleteAction{&oryAction{step: "post-delete"}, &oryFinalizersMock}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		kubeClient.AssertCalled(t, "Clientset")
		kubeClient.AssertCalled(t, "Kubeconfig")
	})

	t.Run("should delete ory JWKS secret when secret exists", func(t *testing.T) {
		// given
		oryFinalizersMock := oryk8smock.OryFinalizersHandler{}
		oryFinalizersMock.On("FindAndDeleteOryFinalizers",
			mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(errors.New("FindAndDeleteOryFinalizers error"))
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		emptyMap := make(map[string]interface{})
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(emptyMap, nil)
		existingSecret := fixSecretJwks()
		clientSet := fake.NewSimpleClientset(existingSecret)
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		_, err := clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, jwksNamespacedName.Name, metav1.GetOptions{})
		require.False(t, kerrors.IsNotFound(err))
		action := postDeleteAction{&oryAction{step: "post-delete"}, &oryFinalizersMock}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		kubeClient.AssertCalled(t, "Clientset")
		kubeClient.AssertCalled(t, "Kubeconfig")
		_, err = clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, jwksNamespacedName.Name, metav1.GetOptions{})
		require.True(t, kerrors.IsNotFound(err))
	})

	t.Run("should delete ory DB secret when secret exists", func(t *testing.T) {
		// given
		oryFinalizersMock := oryk8smock.OryFinalizersHandler{}
		oryFinalizersMock.On("FindAndDeleteOryFinalizers",
			mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(errors.New("FindAndDeleteOryFinalizers error"))
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		values, err := unmarshalTestValues(postgresqlYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		existingSecret := fixSecretMemory()
		clientSet := fake.NewSimpleClientset(existingSecret)
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		_, err = clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.False(t, kerrors.IsNotFound(err))
		action := postDeleteAction{&oryAction{step: "post-delete"}, &oryFinalizersMock}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		kubeClient.AssertCalled(t, "Clientset")
		kubeClient.AssertCalled(t, "Kubeconfig")
		_, err = clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.True(t, kerrors.IsNotFound(err))
	})

	t.Run("should skip deletion of Hydra DB secret when hydra is disabled", func(t *testing.T) {
		// given
		oryFinalizersMock := oryk8smock.OryFinalizersHandler{}
		oryFinalizersMock.On("FindAndDeleteOryFinalizers",
			mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(errors.New("FindAndDeleteOryFinalizers error"))
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		values, err := unmarshalTestValues(hydraDisabledYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		existingSecret := fixSecretMemory()
		clientSet := fake.NewSimpleClientset(existingSecret)
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)

		action := postDeleteAction{&oryAction{step: "post-delete"}, &oryFinalizersMock}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		kubeClient.AssertCalled(t, "Clientset")
		kubeClient.AssertCalled(t, "Kubeconfig")
		_, err = clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.NoError(t, err)
	})

	t.Run("should skip deletion of Hydra DB secret when hydra is not in config", func(t *testing.T) {
		// given
		oryFinalizersMock := oryk8smock.OryFinalizersHandler{}
		oryFinalizersMock.On("FindAndDeleteOryFinalizers",
			mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).
			Return(errors.New("FindAndDeleteOryFinalizers error"))
		factory := chartmocks.Factory{}
		provider := chartmocks.Provider{}
		values, err := unmarshalTestValues(hydraNotExistentYaml)
		require.NoError(t, err)
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(values, nil)
		existingSecret := fixSecretMemory()
		clientSet := fake.NewSimpleClientset(existingSecret)
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)

		action := postDeleteAction{&oryAction{step: "post-delete"}, &oryFinalizersMock}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		kubeClient.AssertCalled(t, "Clientset")
		kubeClient.AssertCalled(t, "Kubeconfig")
		_, err = clientSet.CoreV1().Secrets(oryNamespace).Get(actionContext.Context, databaseSecretName, metav1.GetOptions{})
		require.NoError(t, err)
	})
}

func TestOryDbSecret(t *testing.T) {
	tests := []struct {
		Name            string
		PreCreateSecret bool
	}{
		{
			Name:            "Ory credentials secret created successfully",
			PreCreateSecret: false,
		},
	}
	for _, testCase := range tests {
		test := testCase
		t.Run(test.Name, func(t *testing.T) {
			logger := zaptest.NewLogger(t).Sugar()
			name := types.NamespacedName{Name: "test-db-secret", Namespace: "test"}
			ctx := context.Background()
			k8sClient := fake.NewSimpleClientset()
			var existingUID types.UID

			component := chart.NewComponentBuilder("main", componentName).
				WithNamespace(name.Namespace).
				WithProfile(profileName).
				Build()

			helm, err := chart.NewHelmClient(chartDir, logger)
			require.NoError(t, err)

			values, err := helm.Configuration(component)
			require.NoError(t, err)

			secretObject, err := db.Get(ctx, k8sClient, name, values, logger)
			require.NoError(t, err)

			if test.PreCreateSecret {
				existingSecret, err := preCreateSecret(ctx, k8sClient, name)
				assert.NoError(t, err)
				existingUID = existingSecret.UID
			}

			err = createSecret(ctx, k8sClient, name, *secretObject, logger)
			assert.NoError(t, err)

			secret, err := k8sClient.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
			if !test.PreCreateSecret {
				require.NoError(t, err)
				assert.Equal(t, name.Name, secret.Name)
				assert.Equal(t, name.Namespace, secret.Namespace)
				assert.NotNil(t, secret.StringData)
				assert.Equal(t, secret.StringData["postgresql-password"], "testerpw")

			} else {
				require.NoError(t, err)
				assert.Equal(t, existingUID, secret.UID)
			}

		})
	}
}
func TestOryDbSecret_Update(t *testing.T) {
	name := types.NamespacedName{Name: "test-db-secret", Namespace: "test"}
	ctx := context.Background()
	logger := zaptest.NewLogger(t).Sugar()
	component := chart.NewComponentBuilder("main", componentName).
		WithNamespace(name.Namespace).
		WithProfile(profileName).
		Build()

	helm, err := chart.NewHelmClient(chartDir, logger)
	require.NoError(t, err)

	values, err := helm.Configuration(component)
	require.NoError(t, err)
	t.Run("should return false when applying the same secret", func(t *testing.T) {
		// given
		k8sClient := fake.NewSimpleClientset()

		dbSecretObject, err := db.Get(ctx, k8sClient, name, values, logger)
		require.NoError(t, err)

		err = createSecret(ctx, k8sClient, name, *dbSecretObject, logger)
		require.NoError(t, err)

		existingSecret, err := getSecret(ctx, k8sClient, name)
		require.NoError(t, err)

		existingSecret.Data = map[string][]byte{
			"secretsSystem":                   []byte(existingSecret.StringData["secretsSystem"]),
			"secretsCookie":                   []byte(existingSecret.StringData["secretsCookie"]),
			"dsn":                             []byte(existingSecret.StringData["dsn"]),
			"postgresql-password":             []byte(existingSecret.StringData["postgresql-password"]),
			"postgresql-replication-password": []byte(existingSecret.StringData["postgresql-replication-password"]),
		}

		newSecretData, err := db.Update(ctx, k8sClient, values, existingSecret, logger)
		require.NoError(t, err)

		// when
		check := isUpdate(newSecretData)

		// then
		assert.NoError(t, err)
		require.Equal(t, false, check)
	})
	t.Run("should return true when applying secret with changes", func(t *testing.T) {
		//given
		k8sClient := fake.NewSimpleClientset()
		testMap := map[string]string{
			"secretsSystem": "system",
			"secretsCookie": "cookie",
			"dsn":           "inMemory",
		}
		existingSecret, err := preCreateSecret(ctx, k8sClient, name)
		require.NoError(t, err)

		existingSecret.StringData = testMap
		newSecretData, err := db.Update(ctx, k8sClient, values, existingSecret, logger)

		// when
		check := isUpdate(newSecretData)

		// then
		assert.NoError(t, err)
		require.Equal(t, true, check)
	})
}
func preCreateSecret(ctx context.Context, client k8s.Interface, name types.NamespacedName) (*v1.Secret, error) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: map[string][]byte{},
	}

	return client.CoreV1().Secrets(name.Namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func newFakeKubeClient(clientSet *fake.Clientset) *k8smocks.Client {
	mockClient := &k8smocks.Client{}
	mockClient.On("Clientset").Return(clientSet, nil)
	mockClient.On("Kubeconfig").Return("kubeconfig")
	return mockClient
}

func newFakeServiceContext(factory chart.Factory, provider chart.Provider, client kubernetes.Client) *service.ActionContext {
	logger := log.NewLogger(true)
	task := reconciler.Task{
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
		Task:             &task,
	}
}

func fixSecretMemory() *v1.Secret {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      databaseSecretName,
			Namespace: oryNamespace,
		},
		Data: map[string][]byte{
			"dsn":           []byte(inMemoryURL),
			"secretsCookie": []byte("somesecretcookie"),
			"secretsSystem": []byte("somesecretsystem"),
		},
	}
	return &secret
}

func getJWKSData() map[string][]byte {
	return map[string][]byte{
		"jwks.json": []byte("randomstring"),
	}
}

func fixSecretJwks() *v1.Secret {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jwksNamespacedName.Name,
			Namespace: oryNamespace,
		},
		Data: getJWKSData(),
	}
	return &secret
}

func fixOryHydraDeployment() *v1apps.Deployment {
	return &v1apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "ory-hydra",
			Namespace:  oryNamespace,
			Generation: 1,
		},
	}
}

func unmarshalTestValues(yamlValues string) (map[string]interface{}, error) {
	var values map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlValues), &values)
	if err != nil {
		return nil, err
	}
	return values, nil
}
