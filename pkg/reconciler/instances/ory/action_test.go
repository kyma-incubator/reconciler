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
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/jwks"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	workspacemocks "github.com/kyma-incubator/reconciler/pkg/reconciler/workspace/mocks"
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
            enabled: false`

	postgresqlYaml = `
    global:
      ory:
        hydra:
          persistence:
            enabled: true
            postgresql:
              enabled: true`
)

const (
	profileName   = "profile"
	componentName = "test-ory"
)

var chartDir = filepath.Join("test", "resources")

func Test_PreInstallAction_Run(t *testing.T) {
	t.Run("should not perform any action when chart configuration returned an error", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(nil, errors.New("Configuration error"))
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to retrieve Ory chart values")
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertNotCalled(t, "Clientset")
	})

	t.Run("should not perform any action when kubernetes clientset returned an error", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		emptyMap := make(map[string]interface{})
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(emptyMap, nil)
		kubeClient := k8smocks.Client{}
		kubeClient.On("Clientset").Return(nil, errors.New("cannot get secret"))
		actionContext := newFakeServiceContext(&factory, &provider, &kubeClient)
		action := preReconcileAction{&oryAction{step: "pre-install"}}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot get secret")
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
	})

	t.Run("should create ory secret when secret does not exist", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
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
		secret, err := clientSet.CoreV1().Secrets(dbNamespacedName.Namespace).Get(actionContext.Context, dbNamespacedName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, dbNamespacedName.Name, secret.Name)
		require.Equal(t, dbNamespacedName.Namespace, secret.Namespace)
		require.Equal(t, "memory", secret.StringData["dsn"])
	})

	t.Run("should not update ory secret when secret exist and has a valid data", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
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
		secret, err := clientSet.CoreV1().Secrets(dbNamespacedName.Namespace).Get(actionContext.Context, dbNamespacedName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, dbNamespacedName.Name, secret.Name)
		require.Equal(t, dbNamespacedName.Namespace, secret.Namespace)
		require.Equal(t, "", secret.StringData["dsn"])
		require.Equal(t, []byte("memory"), secret.Data["dsn"])
	})

	t.Run("should update ory secret when secret exist and has an outdated values", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
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
		secret, err := clientSet.CoreV1().Secrets(dbNamespacedName.Namespace).Get(actionContext.Context, dbNamespacedName.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, dbNamespacedName.Name, secret.Name)
		require.Equal(t, dbNamespacedName.Namespace, secret.Namespace)
		require.Contains(t, secret.StringData["dsn"], "postgres")
		require.NotContains(t, secret.StringData["dsn"], "memory")
	})
}

func Test_PreDeleteAction_Run(t *testing.T) {
	t.Run("should not perform any action when kubernetes clientset returned an error", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		kubeClient := k8smocks.Client{}
		kubeClient.On("Clientset").Return(nil, errors.New("failed to retrieve native Kubernetes GO client"))
		actionContext := newFakeServiceContext(&factory, &provider, &kubeClient)
		action := preDeleteAction{&oryAction{step: "pre-delete"}}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to retrieve native Kubernetes GO client")
		kubeClient.AssertCalled(t, "Clientset")
	})

	t.Run("should not perform any action when getting secret returns an error", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		kubeClient := k8smocks.Client{}
		kubeClient.On("Clientset").Return(nil, errors.New("Could not get DB secret"))
		actionContext := newFakeServiceContext(&factory, &provider, &kubeClient)
		action := preDeleteAction{&oryAction{step: "pre-delete"}}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Could not get DB secret")
		kubeClient.AssertCalled(t, "Clientset")
	})

	t.Run("should not perform any action when secret does not exist", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		clientSet := fake.NewSimpleClientset()
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preDeleteAction{&oryAction{step: "pre-delete"}}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		kubeClient.AssertCalled(t, "Clientset")
	})

	t.Run("should delete ory secret when secret exists", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		existingSecret := fixSecretMemory()
		clientSet := fake.NewSimpleClientset(existingSecret)
		kubeClient := newFakeKubeClient(clientSet)
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		_, err := clientSet.CoreV1().Secrets(dbNamespacedName.Namespace).Get(actionContext.Context, dbNamespacedName.Name, metav1.GetOptions{})
		require.False(t, kerrors.IsNotFound(err))
		action := preDeleteAction{&oryAction{step: "pre-delete"}}

		// when
		err = action.Run(actionContext)

		// then
		require.NoError(t, err)
		kubeClient.AssertCalled(t, "Clientset")
		_, err = clientSet.CoreV1().Secrets(dbNamespacedName.Namespace).Get(actionContext.Context, dbNamespacedName.Name, metav1.GetOptions{})
		require.True(t, kerrors.IsNotFound(err))
	})
}

func TestOryJwksSecret(t *testing.T) {
	tests := []struct {
		Name            string
		PreCreateSecret bool
	}{
		{
			Name:            "Secret to patch does not exist",
			PreCreateSecret: false,
		},
		{
			Name:            "Secret was patched successfully",
			PreCreateSecret: true,
		},
	}
	for _, testCase := range tests {
		test := testCase
		t.Run(test.Name, func(t *testing.T) {
			logger := zaptest.NewLogger(t).Sugar()
			a := postReconcileAction{
				&oryAction{step: "test-jwks-secret"},
			}
			name := types.NamespacedName{Name: "test-jwks-secret", Namespace: "test"}
			ctx := context.Background()
			k8sClient := fake.NewSimpleClientset()
			var existingUID types.UID

			patchData, err := jwks.Get(jwksAlg, jwksBits)
			require.NoError(t, err)

			if test.PreCreateSecret {
				existingSecret, err := preCreateSecret(ctx, k8sClient, name)
				assert.NoError(t, err)
				existingUID = existingSecret.UID
				require.Equal(t, true, isEmpty(existingSecret))
			}

			err = a.patchSecret(ctx, k8sClient, name, patchData, logger)
			if !test.PreCreateSecret {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)

				secret, err := k8sClient.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, name.Name, secret.Name)
				assert.Equal(t, name.Namespace, secret.Namespace)
				assert.NotNil(t, secret.Data)
				assert.Equal(t, existingUID, secret.UID)
			}

		})
	}
}
func TestOryJwksSecret_IsEmpty(t *testing.T) {
	t.Run("should return true on empty Secret", func(t *testing.T) {
		// given
		name := types.NamespacedName{Name: "test-jwks-secret", Namespace: "test"}
		ctx := context.Background()
		k8sClient := fake.NewSimpleClientset()
		existingSecret, err := preCreateSecret(ctx, k8sClient, name)

		// when
		check := isEmpty(existingSecret)

		// then
		assert.NoError(t, err)
		require.Equal(t, true, check)
	})
	t.Run("should return false on non-empty Secret", func(t *testing.T) {
		// given
		name := types.NamespacedName{Name: "test-jwks-secret", Namespace: "test"}
		ctx := context.Background()
		k8sClient := fake.NewSimpleClientset()
		existingSecret, err := preCreateSecret(ctx, k8sClient, name)
		existingSecret.Data = map[string][]byte{"jwks.json": []byte("test")}

		// when
		check := isEmpty(existingSecret)

		// then
		assert.NoError(t, err)
		require.Equal(t, false, check)
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

			secretObject, err := db.Get(name, values, logger)
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
	mockClient.On("Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	return mockClient
}

func newFakeServiceContext(factory workspace.Factory, provider chart.Provider, client kubernetes.Client) *service.ActionContext {
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
			Name:      dbNamespacedName.Name,
			Namespace: dbNamespacedName.Namespace,
		},
		Data: map[string][]byte{
			"dsn":           []byte("memory"),
			"secretsCookie": []byte("somesecretcookie"),
			"secretsSystem": []byte("somesecretsystem"),
		},
	}
	return &secret
}

func fixOryHydraDeployment() *v1apps.Deployment {
	return &v1apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "ory-hydra",
			Namespace:  dbNamespacedName.Namespace,
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
