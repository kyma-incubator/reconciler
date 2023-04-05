package connectivityproxy

import (
	"context"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
	apiCoreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestLoader(t *testing.T) {

	secretName := "test-secret-name"
	namespace := "default"

	client := &mocks.Client{}
	secret := &apiCoreV1.Secret{}

	background := context.Background()
	actionContext := &service.ActionContext{
		Context: background,
		Task: &reconciler.Task{
			Namespace: namespace,
		},
		KubeClient: client,
		Logger:     logger.NewLogger(true),
	}

	loader := K8sLoader{}

	t.Run("Should find secret from unstructured binding with default namespace", func(t *testing.T) {
		client.On("GetSecret", background, secretName, namespace).Return(secret, nil)

		result, err := loader.FindSecret(actionContext, &unstructured.Unstructured{Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"secretName": secretName,
			},
		}})

		require.NoError(t, err)
		require.Equal(t, secret, result)
	})

	t.Run("Should find secret from unstructured binding with its namespace", func(t *testing.T) {

		namespace := "test-namespace"
		actionContext.Task.Namespace = namespace

		client.On("GetSecret", background, secretName, namespace).Return(secret, nil)

		result, err := loader.FindSecret(actionContext, &unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"namespace": "test-namespace",
			},
			"spec": map[string]interface{}{
				"secretName": secretName,
			},
		}})

		require.NoError(t, err)
		require.Equal(t, secret, result)
	})
}
