package serverless

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	pmock "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	rkubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func setup() (kubernetes.Interface, *service.ActionContext) {
	k8sClient := fake.NewSimpleClientset()

	mockClient := mocks.Client{}
	mockClient.On("Clientset").Return(k8sClient, nil)
	mockClient.On("Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*rkubernetes.Resource{}, nil)
	configuration := map[string]interface{}{}
	mockProvider := pmock.Provider{}
	mockManifest := chart.Manifest{
		Manifest: "",
	}
	mockProvider.On("RenderManifest", mock.Anything).Return(&mockManifest, nil)

	actionContext := &service.ActionContext{
		KubeClient:    &mockClient,
		Context:       context.TODO(),
		Logger:        logger.NewLogger(false),
		ChartProvider: &mockProvider,
		Task:          &reconciler.Task{Version: "test", Configuration: configuration},
	}
	return k8sClient, actionContext
}
