package preaction

import (
	"context"
	pmock "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes/fake"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/stretchr/testify/require"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func TestDeletingNatsOperatorResources(t *testing.T) {
	setup := func() (removeNatsOperatorStep, *service.ActionContext, *pmock.Provider, *mocks.Client) {
		mainContext := context.TODO()
		k8sClient := mocks.Client{}
		log := logger.NewLogger(false)

		mockProvider := pmock.Provider{}
		mockManifest := chart.Manifest{
			Manifest: "testManifest",
		}
		mockProvider.On("RenderManifest", mock.Anything).Return(&mockManifest, nil)

		// mock the delete calls
		k8sClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
		k8sClient.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return([]*kubernetes.Resource{}, nil)
		k8sClient.On(
			"DeleteResourceByKindAndNameAndNamespace",
			"customresourcedefinitions",
			natsOperatorCRDsToDelete[0],
			"kyma-system",
		).Return(nil, nil)
		k8sClient.On(
			"DeleteResourceByKindAndNameAndNamespace",
			"customresourcedefinitions",
			natsOperatorCRDsToDelete[1],
			"kyma-system",
		).Return(nil, nil)

		action := removeNatsOperatorStep{
			kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error) {
				return &k8sClient, nil
			},
		}

		actionContext := &service.ActionContext{
			Context:       mainContext,
			Logger:        log,
			Task:          &reconciler.Task{},
			ChartProvider: &mockProvider,
		}
		return action, actionContext, &mockProvider, &k8sClient
	}

	action, actionContext, mockProvider, k8sClient := setup()

	// execute the step
	err := action.Execute(actionContext, actionContext.Logger)
	require.NoError(t, err)

	// ensure the right calls were invoked
	mockProvider.AssertCalled(t, "RenderManifest", mock.Anything)
	k8sClient.AssertCalled(t, "Delete", actionContext.Context, mock.Anything, namespace)
	k8sClient.AssertCalled(t, "DeleteResourceByKindAndNameAndNamespace", "customresourcedefinitions", natsOperatorCRDsToDelete[0], "kyma-system")
	k8sClient.AssertCalled(t, "DeleteResourceByKindAndNameAndNamespace", "customresourcedefinitions", natsOperatorCRDsToDelete[1], "kyma-system")

}
