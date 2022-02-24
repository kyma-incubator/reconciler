package preaction

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	pmock "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRemovingTheStatefulSet(t *testing.T) {

	var testCases = []struct {
		name                            string
		givenJetstreamFlag              bool
		givenJetstreamDataInStatefulSet bool
		wantStatefulSetDeletion         bool
	}{
		{
			name:                            "When Nats StatefulSet has jetstream data and jetstream is disabled the StatefulSet should remain untouched",
			givenJetstreamFlag:              true,
			givenJetstreamDataInStatefulSet: true,
			wantStatefulSetDeletion:         false,
		},
		{
			name:                            "When Nats StatefulSet has jetstream data but jetstream is disabled the StatefulSet should be deleted",
			givenJetstreamFlag:              false,
			givenJetstreamDataInStatefulSet: true,
			wantStatefulSetDeletion:         true,
		},
		{
			name:                            "When Nats StatefulSet has no jetstream data and jetstream is disabled, the StatefulSet should remain untouched",
			givenJetstreamFlag:              false,
			givenJetstreamDataInStatefulSet: false,
			wantStatefulSetDeletion:         false,
		},
		{
			name:                            "When Nats StatefulSet has jetstream data but jetstream is disabled, the StatefulSet should remain untouched, as the data will be deleted by the reconciler",
			givenJetstreamFlag:              true,
			givenJetstreamDataInStatefulSet: false,
			wantStatefulSetDeletion:         false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			k8sClient, action, actionContext := setup(getHelmEventingValues(tc.givenJetstreamFlag))
			// given
			_, err := k8sClient.AppsV1().StatefulSets(namespace).Create(actionContext.Context, createStatefulSet(tc.givenJetstreamDataInStatefulSet), metav1.CreateOptions{})
			require.NoError(t, err)

			// when
			err = action.Execute(actionContext, actionContext.Logger)
			require.NoError(t, err)

			// then
			gotStatefulSet, err := getStatefulSetUsingClientSet(actionContext, k8sClient, statefulSetName)
			require.NoError(t, err)
			if tc.wantStatefulSetDeletion {
				require.Nil(t, gotStatefulSet)
			} else {
				require.NotNil(t, gotStatefulSet)
			}
		})
	}
}

func setup(chartValues string) (kubernetes.Interface, handleEnablingJetstream, *service.ActionContext) {
	k8sClient := fake.NewSimpleClientset()
	action := handleEnablingJetstream{}
	mockClient := mocks.Client{}
	mockClient.On("Clientset").Return(k8sClient, nil)

	mockProvider := pmock.Provider{}

	mockConfiguration := map[string]interface{}{
		"global": chartValues,
	}
	mockedComponentBuilder := GetResourcesFromVersion(chartsVersion, chartsName)
	mockProvider.On("Configuration", mockedComponentBuilder).Return(mockConfiguration, nil)

	// todo mock manifest with chart values
	actionContext := &service.ActionContext{
		KubeClient:    &mockClient,
		Context:       context.TODO(),
		ChartProvider: &mockProvider,
		Logger:        logger.NewLogger(false),
		Task:          &reconciler.Task{Version: "test"},
	}
	return k8sClient, action, actionContext
}

func createStatefulSet(withJestreamData bool) *v1.StatefulSet {
	var containerEnvs []corev1.EnvVar
	if withJestreamData {
		containerEnvs = []corev1.EnvVar{
			{
				Name:  "JS_ENABLED",
				Value: "true",
			}}
	}
	return &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: namespace,
		},
		Spec: v1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: natsContainerName,
							Env:  containerEnvs,
						},
					},
				},
			},
		},
	}
}

func getHelmEventingValues(enableJetstream bool) string {
	return fmt.Sprintf("{\"configMap\":{\"keys\":{\"eventTypePrefix\":\"eventTypePrefix\"},\"name\":\"eventing\"},\"containerRegistry\":{\"path\":\"eu.gcr.io/kyma-project\"},\"domainName\":\"kyma.example.com\",\"eventTypePrefix\":\"sap.kyma.custom\",\"features\":{\"enableJetStream\":%s},\"image\":{\"repository\":\"eu.gcr.io/kyma-project\"},\"images\":{\"eventing_controller\":{\"name\":\"eventing-controller\",\"pullPolicy\":\"IfNotPresent\",\"version\":\"PR-13464\"},\"nats\":{\"directory\":\"external\",\"name\":\"nats\",\"version\":\"2.6.5-alpine\"},\"publisher_proxy\":{\"name\":\"event-publisher-proxy\",\"version\":\"PR-13469\"}},\"istio\":{\"proxy\":{\"portName\":\"http-status\",\"statusPort\":15020}},\"log\":{\"format\":\"json\",\"level\":\"info\"},\"secretName\":\"\",\"securityContext\":{\"allowPrivilegeEscalation\":false,\"privileged\":false}}", strconv.FormatBool(enableJetstream))
}
