package connectivityproxy_test

import (
	"testing"

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
	"k8s.io/client-go/kubernetes/fake"
)

func TestAction(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	kubeClient := &kubeMocks.Client{}
	kubeClient.On("Clientset").Return(clientset, nil)

	context := &service.ActionContext{
		KubeClient:       kubeClient,
		WorkspaceFactory: nil,
		Context:          nil,
		Logger:           logger.NewLogger(true),
		ChartProvider:    nil,
		Task: &reconciler.Task{
			Component: "test-component",
		},
	}

	loader := &connectivityproxymocks.Loader{}
	commands := &connectivityproxymocks.Commands{}
	binding := &unstructured.Unstructured{}
	secret := &v1.Secret{}
	statefulset := &v1apps.StatefulSet{}

	action := connectivityproxy.CustomAction{
		Name:     "test-name",
		Loader:   loader,
		Commands: commands,
	}

	t.Run("Should install app if binding exists and app is missing - operator", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(nil, nil)
		loader.On("FindBindingOperator", context).Return(binding, nil)
		loader.On("FindSecret", context, binding).Return(secret, nil)

		commands.On("CopyResources", context).Return(nil)
		commands.On("Install", context, secret).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should install app if binding exists and app is missing - catalog", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(nil, nil)
		loader.On("FindBindingOperator", context).Return(nil, nil)
		loader.On("FindBindingCatalog", context).Return(binding, nil)
		loader.On("FindSecret", context, binding).Return(secret, nil)

		commands.On("CopyResources", context).Return(nil)
		commands.On("Install", context, secret).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should remove app if binding is missing and app is existing", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(statefulset, nil)
		loader.On("FindBindingOperator", context).Return(nil, nil)
		loader.On("FindBindingCatalog", context).Return(nil, nil)

		commands.On("Remove", context).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should do nothing if binding and app exists ", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(statefulset, nil)
		loader.On("FindBindingOperator", context).Return(binding, nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should do nothing if binding and app missing ", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(statefulset, nil)
		loader.On("FindBindingOperator", context).Return(nil, nil)
		loader.On("FindBindingCatalog", context).Return(nil, nil)

		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("Should install when secret not found", func(t *testing.T) {
		kubeClient.On("GetStatefulSet", context.Context, "test-component", "").
			Return(nil, nil)

		loader.On("FindBindingOperator", context).Return(binding, nil)

		loader.On("FindSecret", context, binding).
			Return(nil, nil)

		commands.On("CopyResources", context).Return(nil)
		commands.On("Install", context, (*v1.Secret)(nil)).Return(nil)

		err := action.Run(context)
		require.NoError(t, err)
	})
}
