package connectivityproxy_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy"
	connectivityproxymocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/mocks"
	kubeMocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func setupActionTestEnv() (*kubeMocks.Client, *service.ActionContext, connectivityproxy.CustomAction, *connectivityproxymocks.Loader, *connectivityproxymocks.Commands) {

	loader := &connectivityproxymocks.Loader{}
	commands := &connectivityproxymocks.Commands{}
	kubeClient := &kubeMocks.Client{}
	action := connectivityproxy.CustomAction{
		Name:     "test-name",
		Loader:   loader,
		Commands: commands,
	}
	context := &service.ActionContext{
		KubeClient:       kubeClient,
		WorkspaceFactory: nil,
		Context:          nil,
		Logger:           logger.NewLogger(true),
		ChartProvider:    nil,
		Task: &reconciler.Task{
			Component: "test-component",
			Configuration: map[string]interface{}{
				"global.binding.url":      "cf.test-address.sap.com",
				"global.binding.CAs_path": "/api/v1/CAs/signing",
			},
		},
	}

	return kubeClient, context, action, loader, commands
}

func TestAction(t *testing.T) {
	// test data
	binding := &unstructured.Unstructured{}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-binding-secret",
			Namespace: "default",
		},
	}
	statefulset := &v1apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-stateful-set",
			Namespace: "default",
		},
	}

	t.Run("Should install app if binding exists and app is missing", func(t *testing.T) {

		kubeClient, context, action, loader, commands := setupActionTestEnv()

		kubeClient.On("GetHost").Return("test host")
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").Return(nil, nil)

		loader.On("FindBindingOperator", context).Return(binding, nil)
		loader.On("FindSecret", context, binding).Return(secret, nil)

		commands.On("CreateCARootSecret", context, mock.AnythingOfType("*connectivityclient.ConnectivityCAClient")).Return(nil)
		commands.On("CreateSecretMappingOperator", context, "kyma-system").Return(nil, nil)
		commands.On("Apply", context, false).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)

		commands.AssertExpectations(t)
		loader.AssertExpectations(t)
		kubeClient.AssertExpectations(t)
	})

	t.Run("Should remove app if binding is missing and app exists", func(t *testing.T) {

		kubeClient, context, action, loader, commands := setupActionTestEnv()

		kubeClient.On("GetHost").Return("test host")
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").Return(statefulset, nil)
		loader.On("FindBindingOperator", context).Return(nil, nil)

		commands.On("Remove", context).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)

		commands.AssertExpectations(t)
		loader.AssertExpectations(t)
		kubeClient.AssertExpectations(t)
	})

	t.Run("Should refresh app when both binding and app exists", func(t *testing.T) {
		kubeClient, context, action, loader, commands := setupActionTestEnv()
		kubeClient.On("Clientset").Return(nil)

		kubeClient.On("GetHost").Return("test host")
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").Return(statefulset, nil)

		commands.On("CreateSecretMappingOperator", context, "kyma-system").Return(nil, nil)

		loader.On("FindBindingOperator", context).Return(binding, nil)
		loader.On("FindSecret", context, binding).Return(secret, nil)

		commands.On("CreateCARootSecret", context, mock.AnythingOfType("*connectivityclient.ConnectivityCAClient")).Return(nil)
		commands.On("Apply", context, true).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
		commands.AssertExpectations(t)
		loader.AssertExpectations(t)
	})

	t.Run("Should do nothing when binding and app are missing ", func(t *testing.T) {
		kubeClient, context, action, loader, commands := setupActionTestEnv()

		kubeClient.On("GetHost").Return("test host")
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").Return(nil, nil)

		loader.On("FindBindingOperator", context).Return(nil, nil)

		err := action.Run(context)
		require.NoError(t, err)

		commands.AssertExpectations(t)
		loader.AssertExpectations(t)
		kubeClient.AssertExpectations(t)
	})

	t.Run("Should ignore error when binding exists, and FindSecret returns error", func(t *testing.T) {
		kubeClient, context, action, loader, commands := setupActionTestEnv()

		kubeClient.On("GetHost").Return("test host")
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").Return(nil, nil)

		loader.On("FindBindingOperator", context).Return(binding, nil)
		loader.On("FindSecret", context, binding).Return(nil, errors.New("some error"))

		err := action.Run(context)
		require.NoError(t, err)

		commands.AssertExpectations(t)
		loader.AssertExpectations(t)
		kubeClient.AssertExpectations(t)
	})
}
