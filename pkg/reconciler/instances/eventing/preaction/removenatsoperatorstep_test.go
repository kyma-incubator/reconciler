package preaction

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	pmock "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func TestDeletingNatsOperatorResources(t *testing.T) {
	var testCases = []struct {
		givenStatefulSet    bool // used to simulate Kyma 2.X Eventing
		givenNatsPodsLength int  // used to simulate orphaned NATS pods from Kyma 1.X
	}{
		{
			givenStatefulSet:    true,
			givenNatsPodsLength: 0,
		},
		{
			givenStatefulSet:    false,
			givenNatsPodsLength: 3,
		},
	}

	for _, tc := range testCases {
		action, actionContext, mockProvider, k8sClient, mockedComponentBuilder, natsPods, err := testSetup(tc.givenStatefulSet, tc.givenNatsPodsLength)
		require.NoError(t, err)

		// execute the step
		err = action.Execute(actionContext, actionContext.Logger)
		require.NoError(t, err)

		// ensure the right calls were invoked
		mockProvider.AssertCalled(t, "RenderManifest", mockedComponentBuilder)
		m := []byte(manifestString)
		us, err := kubernetes.ToUnstructured(m, true)
		require.NoError(t, err)
		for _, u := range us {
			if tc.givenStatefulSet && u.GetName() == eventingNats && strings.EqualFold(u.GetKind(), serviceKind) {
				k8sClient.AssertNotCalled(t, "DeleteResource", actionContext.Context, u.GetKind(), u.GetName(), namespace)
				continue
			}
			k8sClient.AssertCalled(t, "DeleteResource", actionContext.Context, u.GetKind(), u.GetName(), namespace)
		}
		deletedNatsPodsLength := 0
		for _, pod := range natsPods.Items {
			if k8sClient.AssertCalled(t, "DeleteResource", actionContext.Context, pod.GetKind(), pod.GetName(), namespace) {
				deletedNatsPodsLength++
			}
		}
		k8sClient.AssertCalled(t, "DeleteResource", actionContext.Context, crdPlural, natsOperatorCRDsToDelete[0], namespace)
		k8sClient.AssertCalled(t, "DeleteResource", actionContext.Context, crdPlural, natsOperatorCRDsToDelete[1], namespace)
		k8sClient.AssertCalled(t, "GetStatefulSet", actionContext.Context, eventingNats, namespace)
		require.Equal(t, tc.givenNatsPodsLength, deletedNatsPodsLength)
	}
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

func testSetup(givenStatefulSet bool, givenNatsPodsLength int) (*removeNatsOperatorStep, *service.ActionContext, *pmock.Provider, *mocks.Client, *chart.Component, *unstructured.UnstructuredList, error) {
	ctx := context.TODO()

	var statefulSet *v1.StatefulSet
	if givenStatefulSet {
		statefulSet = &v1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: eventingNats, Namespace: namespace}}
	}

	natsPods := unstructuredNatsPods(givenNatsPodsLength)

	// setup mock client
	k8sClient := mocks.Client{}

	resources, err := kubernetes.ToUnstructured([]byte(manifestString), true)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	for _, resource := range resources {
		k8sClient.On("DeleteResource", ctx, resource.GetKind(), resource.GetName(), namespace).Return(nil, nil)
	}

	for _, natsPod := range natsPods.Items {
		k8sClient.On("DeleteResource", ctx, natsPod.GetKind(), natsPod.GetName(), namespace).Return(nil, nil)
	}

	k8sClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
	k8sClient.On("GetStatefulSet", ctx, eventingNats, namespace).Return(statefulSet, nil)
	k8sClient.On("DeleteResource", ctx, crdPlural, natsOperatorCRDsToDelete[0], namespace).Return(nil, nil)
	k8sClient.On("DeleteResource", ctx, crdPlural, natsOperatorCRDsToDelete[1], namespace).Return(nil, nil)
	k8sClient.On("ListResource", ctx, podKind, metav1.ListOptions{LabelSelector: getNatsPodLabelSelector()}).Return(natsPods, nil)

	action := removeNatsOperatorStep{
		kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error) {
			return &k8sClient, nil
		},
	}

	mockProvider := pmock.Provider{}
	mockManifest := chart.Manifest{Manifest: manifestString}
	mockedComponentBuilder := GetResourcesFromVersion(natsOperatorLastVersion, natsSubChartPath)
	mockProvider.On("RenderManifest", mockedComponentBuilder).Return(&mockManifest, nil)

	actionContext := &service.ActionContext{
		Context:       ctx,
		Logger:        logger.NewLogger(false),
		KubeClient:    &k8sClient,
		ChartProvider: &mockProvider,
		Task:          &reconciler.Task{},
	}

	return &action, actionContext, &mockProvider, &k8sClient, mockedComponentBuilder, natsPods, nil
}

func unstructuredNatsPods(length int) *unstructured.UnstructuredList {
	const natsPodNameFormat = "nats-%d"
	instances := make([]unstructured.Unstructured, length)
	for i := 0; i < length; i++ {
		instances[i].Object = map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf(natsPodNameFormat, i),
				"namespace": namespace,
				"labels":    getNatsPodLabels(),
			},
		}
	}
	return &unstructured.UnstructuredList{Items: instances}
}
