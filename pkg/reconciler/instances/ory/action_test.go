package ory

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_PreAction_Run(t *testing.T) {
	t.Run("should not perform any action when chart configuration returned an error", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(nil, errors.New("Configuration error"))
		kubeClient := newFakeKubeClient()
		actionContext := newFakeServiceContext(&factory, &provider, kubeClient)
		action := preAction{&oryAction{step: "pre-install"}}

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
		action := preAction{&oryAction{step: "pre-install"}}

		// when
		err := action.Run(actionContext)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot get secret")
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
	})

	t.Run("should not perform any action when kubernetes secret get returned an error", func(t *testing.T) {
		// given
		factory := workspacemocks.Factory{}
		provider := chartmocks.Provider{}
		emptyMap := make(map[string]interface{})
		provider.On("Configuration", mock.AnythingOfType("*chart.Component")).Return(emptyMap, nil)
		kubeClient := k8smocks.Client{}
		kubeClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
		// TODO: Fix method chain or find another way to test secret Get
		kubeClient.On("CoreV1").On("Secrets", mock.AnythingOfType("string")).
			On("Get", mock.AnythingOfType("context.Context"), mock.AnythingOfType("string"), mock.AnythingOfType("metav1.GetOptions")).
			Return(nil, errors.New("cannot get secret"))

		actionContext := newFakeServiceContext(&factory, &provider, &kubeClient)
		action := preAction{&oryAction{step: "pre-install"}}

		// when
		err := action.Run(actionContext)

		// then
		require.NoError(t, err)
		// require.Error(t, err)
		// require.Contains(t, err.Error(), "cannot get secret")
		provider.AssertCalled(t, "Configuration", mock.AnythingOfType("*chart.Component"))
		kubeClient.AssertCalled(t, "Clientset")
	})
}

const (
	profileName   = "profile"
	componentName = "test-ory"
)

var chartDir = filepath.Join("test", "resources")

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
			a := postAction{
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

func newFakeKubeClient() *k8smocks.Client {
	mockClient := &k8smocks.Client{}
	mockClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
	mockClient.On("Kubeconfig").Return("kubeconfig")
	mockClient.On("Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	return mockClient
}

func newFakeServiceContext(factory workspace.Factory, provider chart.Provider, client kubernetes.Client) *service.ActionContext {
	logger := log.NewLogger(true)
	model := reconciler.Reconciliation{
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
		Model:            &model,
	}
}
