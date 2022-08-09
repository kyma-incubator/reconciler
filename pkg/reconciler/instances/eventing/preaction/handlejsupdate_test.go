package preaction

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"testing"

	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/require"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func TestJsUpdate(t *testing.T) {
	testCases := []struct {
		name                    string
		givenStatefulSet        *appsv1.StatefulSet
		wantStatefulSetDeletion bool
	}{
		{
			name:                    "Should delete the StatefulSet if the SERVER_NAME env variable is not present",
			givenStatefulSet:        newStatefulSet(),
			wantStatefulSetDeletion: true,
		},
		{
			name:                    "Should do nothing if the SERVER_NAME env variable is present",
			givenStatefulSet:        newStatefulSet(withEnvVar(v1.EnvVar{Name: serverNameEnv, Value: "true"})),
			wantStatefulSetDeletion: false,
		},
	}

	setup := func(t *testing.T) (kubernetes.Interface, handleJsUpdate, *service.ActionContext) {
		k8sClient := fake.NewSimpleClientset()
		mockClient := mocks.Client{}
		mockClient.On("Clientset").Return(k8sClient, nil)
		action := handleJsUpdate{
			kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (k8s.Client, error) {
				return &mockClient, nil
			},
		}

		chartProvider := &chartmocks.Provider{}
		chartValuesYAML := getJetstreamValuesYAML(true, fileStorageType)
		chartValues, err := unmarshalTestValues(chartValuesYAML)
		require.NoError(t, err)

		chartProvider.On("Configuration", mock.Anything).Return(chartValues, nil)

		actionContext := &service.ActionContext{
			KubeClient:    &mockClient,
			Context:       context.TODO(),
			Logger:        logger.NewLogger(false),
			Task:          &reconciler.Task{Version: "test"},
			ChartProvider: chartProvider,
		}
		return k8sClient, action, actionContext
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var err error
			k8sClient, action, actionContext := setup(t)

			_, err = createStatefulSet(actionContext.Context, k8sClient, tc.givenStatefulSet)
			require.NoError(t, err)

			// when
			err = action.Execute(actionContext, log.ContextLogger(actionContext, log.WithAction(actionName)))
			require.NoError(t, err)

			// then
			gotStatefulSet, err := k8sClient.AppsV1().StatefulSets(namespace).Get(actionContext.Context, statefulSetName, metav1.GetOptions{})
			if !k8serrors.IsNotFound(err) {
				require.NoError(t, err)
			}
			if tc.wantStatefulSetDeletion {
				require.Nil(t, gotStatefulSet)
			} else {
				require.NotNil(t, gotStatefulSet)
			}
		})
	}
}
