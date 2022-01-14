package preaction

import (
	"context"
	pmock "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
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

const (
	manifestString = "testManifest"
	//kyma1xVersion  = "1.24.8"
	//kyma2xVersion  = "2.0"
)

func TestDeletingNatsOperatorResources(t *testing.T) {
	action, actionContext, mockProvider, k8sClient, mockedComponentBuilder := testSetup()

	// execute the step
	err := action.Execute(actionContext, actionContext.Logger)
	require.NoError(t, err)

	// ensure the right calls were invoked
	mockProvider.AssertCalled(t, "RenderManifest", mockedComponentBuilder)
	k8sClient.AssertCalled(t, "Delete", actionContext.Context, manifestString, namespace)
	k8sClient.AssertCalled(t, "DeleteResource", actionContext.Context, crdPlural, natsOperatorCRDsToDelete[0], namespace)
	k8sClient.AssertCalled(t, "DeleteResource", actionContext.Context, crdPlural, natsOperatorCRDsToDelete[1], namespace)
}

// todo execute this test, when the check for kyma2x version is available, see the the todo comment from removenatsoperatorstep:Execute()
//func TestSkippingNatsOperatorDeletionFox2x(t *testing.T) {
//	action, actionContext, mockProvider, k8sClient, _ := testSetup(kyma2xVersion)
//
//	// execute the step
//	err := action.Execute(actionContext, actionContext.Logger)
//	require.NoError(t, err)
//
//	mockProvider.AssertNotCalled(t, "RenderManifest", mock.Anything)
//	k8sClient.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
//	k8sClient.AssertNotCalled(t, "DeleteResource", mock.Anything, mock.Anything, mock.Anything)
//	k8sClient.AssertNotCalled(t, "DeleteResource", mock.Anything, mock.Anything, mock.Anything)
//}

func testSetup() (removeNatsOperatorStep, *service.ActionContext, *pmock.Provider, *mocks.Client, *chart.Component) {
	ctx := context.TODO()
	k8sClient := mocks.Client{}
	log := logger.NewLogger(false)

	mockProvider := pmock.Provider{}
	mockManifest := chart.Manifest{
		Manifest: manifestString,
	}
	action := removeNatsOperatorStep{
		kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error) {
			return &k8sClient, nil
		},
	}
	mockedComponentBuilder := GetResourcesFromVersion(natsOperatorLastVersion, natsSubChartPath)
	mockProvider.On("RenderManifest", mockedComponentBuilder).Return(&mockManifest, nil)

	// mock the delete calls
	k8sClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
	k8sClient.On("Delete", ctx, manifestString, namespace).Return([]*kubernetes.Resource{}, nil)
	k8sClient.On(
		"DeleteResource",
		ctx,
		crdPlural,
		natsOperatorCRDsToDelete[0],
		namespace,
	).Return(nil, nil)
	k8sClient.On(
		"DeleteResource",
		ctx,
		crdPlural,
		natsOperatorCRDsToDelete[1],
		namespace,
	).Return(nil, nil)

	actionContext := &service.ActionContext{
		Context:       ctx,
		Logger:        log,
		ChartProvider: &mockProvider,
		Task:          &reconciler.Task{},
	}
	return action, actionContext, &mockProvider, &k8sClient, mockedComponentBuilder
}
