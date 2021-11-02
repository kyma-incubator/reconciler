package connectivityproxy

import (
	"context"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
	apiCoreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestLoader(t *testing.T) {

	t.Run("Should find secret from unstructured binding", func(t *testing.T) {
		background := context.Background()

		secretName := "test-secret-name"
		namespace := "default"

		client := &mocks.Client{}
		secret := &apiCoreV1.Secret{}
		client.On("GetSecret", background, secretName, namespace).Return(secret, nil)

		actionContext := &service.ActionContext{
			Context: background,
			Task: &reconciler.Task{
				Namespace: namespace,
			},
			KubeClient: client,
		}

		loader := K8sLoader{}

		result, err := loader.FindSecret(actionContext, &unstructured.Unstructured{Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"secretName": secretName,
			},
		}})

		require.NoError(t, err)
		require.Equal(t, secret, result)
	})
}
