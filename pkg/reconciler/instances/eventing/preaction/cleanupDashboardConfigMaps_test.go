package preaction

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestCleanupDashboardConfigMapsPreAction(t *testing.T) {
	testCases := []struct {
		name                    string
		givenConfigMaps         []string
		wantConfigMapsUntouched []string
	}{
		{
			name:                    "Should do nothing if there are no ConfigMaps",
			givenConfigMaps:         nil,
			wantConfigMapsUntouched: nil,
		},
		{
			name:                    "Should delete both ConfigMaps",
			givenConfigMaps:         []string{"eventing-dashboards-event-types-summary", "eventing-dashboards-delivery-per-subscription"},
			wantConfigMapsUntouched: nil,
		},
		{
			name:                    "Should delete one and retain the other ConfigMap",
			givenConfigMaps:         []string{"eventing-dashboards-event-types-summary", "eventing-dashboards-jetstream"},
			wantConfigMapsUntouched: []string{"eventing-dashboards-jetstream"},
		},
	}

	setupTestEnvironment := func(t *testing.T, configMaps []string) (*fake.Clientset, handleCleanupDashboardConfigMaps, *service.ActionContext) {
		k8sClient := fake.NewSimpleClientset()
		mockClient := mocks.Client{}
		mockClient.On("Clientset").Return(k8sClient, nil)
		action := handleCleanupDashboardConfigMaps{
			kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (k8s.Client, error) {
				return &mockClient, nil
			},
		}
		chartProvider := &chartmocks.Provider{}
		ctx := context.TODO()
		for _, configMapName := range configMaps {
			err := createConfigMap(ctx, k8sClient, configMapName)
			require.NoError(t, err)
		}

		actionContext := &service.ActionContext{
			KubeClient:    &mockClient,
			Context:       ctx,
			Logger:        logger.NewLogger(false),
			Task:          &reconciler.Task{Version: "test"},
			ChartProvider: chartProvider,
		}
		return k8sClient, action, actionContext
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			// given
			k8sClient, action, actionContext := setupTestEnvironment(t, tc.givenConfigMaps)

			// when
			err := action.Execute(actionContext, log.ContextLogger(actionContext, log.WithAction(actionName)))

			// then
			require.NoError(t, err)
			err = checkConfigMapExists(actionContext.Context, k8sClient, tc.wantConfigMapsUntouched)
			require.NoError(t, err)
		})
	}
}

func createConfigMap(ctx context.Context, client *fake.Clientset, name string) error {
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if _, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

func checkConfigMapExists(ctx context.Context, client *fake.Clientset, configMaps []string) error {
	for _, configMapName := range configMaps {
		if _, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{}); err != nil {
			return err
		}
	}
	return nil
}
