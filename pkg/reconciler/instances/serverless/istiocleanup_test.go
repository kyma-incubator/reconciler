package serverless

import (
	"context"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResourceCleanupAction_Run(t *testing.T) {
	t.Run("cleanup resources", func(t *testing.T) {
		ctx := context.Background()
		mockClient := &mocks.Client{}

		mockClient.On("DeleteResource", ctx, "testKind-1", "testName-1", "testNamespace-1").Return(nil, nil)
		mockClient.On("DeleteResource", ctx, "testKind-2", "testName-2", "testNamespace-2").Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))

		context := &service.ActionContext{
			Context:    ctx,
			Logger:     zap.NewNop().Sugar(),
			KubeClient: mockClient,
		}
		action := &ResourceCleanupAction{
			resources: []kubernetes.Resource{
				{Kind: "testKind-1", Name: "testName-1", Namespace: "testNamespace-1"},
				{Kind: "testKind-2", Name: "testName-2", Namespace: "testNamespace-2"},
			},
		}

		err := action.Run(context)
		require.NoError(t, err)
	})
	t.Run("handle error", func(t *testing.T) {
		ctx := context.Background()
		mockClient := &mocks.Client{}

		mockClient.On("DeleteResource", ctx, "testKind-1", "testName-1", "testNamespace-1").Return(nil, errors.NewBadRequest("client error"))

		context := &service.ActionContext{
			Context:    ctx,
			Logger:     zap.NewNop().Sugar(),
			KubeClient: mockClient,
		}
		action := &ResourceCleanupAction{
			resources: []kubernetes.Resource{
				{Kind: "testKind-1", Name: "testName-1", Namespace: "testNamespace-1"},
			},
		}

		err := action.Run(context)
		require.Error(t, err)
	})
}
