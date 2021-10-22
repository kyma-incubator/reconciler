package serverless

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mock "github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	pmock "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	rkubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func TestServerlessReconcilation(t *testing.T) {

	testCases := []struct {
		name                             string
		existingSecret                   *corev1.Secret
		existingDockerRegistryDeployment *appsv1.Deployment
		expectedReconcilerConfiguration  map[string]interface{}
	}{
		{
			name:                            "No docker registry secret found",
			expectedReconcilerConfiguration: map[string]interface{}{},
		},
		//  {
		//  	name:                            "Secret found",
		//  	existingSecret: ,
		//  	expectedReconcilerConfiguration: map[string]interface{}{
		//  		"dockerRegistry.username" : "",
		//  		"dockerRegistry.password" : "",
		//  	},
		//  },
	}

	setup := func() (kubernetes.Interface, ServerlessReconcileCustomAction, *service.ActionContext) {
		k8sClient := fake.NewSimpleClientset()

		action := ServerlessReconcileCustomAction{}
		mockClient := mocks.Client{}
		mockClient.On("Clientset").Return(k8sClient, nil)
		mockClient.On("Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*rkubernetes.Resource{}, nil)
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
			Model:         &reconciler.Reconciliation{Version: "test", Configuration: configuration},
		}
		return k8sClient, action, actionContext
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			k8sClient, action, actionContext := setup()

			if tc.existingSecret != nil {
				_, err := createSecret(actionContext.Context, k8sClient, tc.existingSecret)
				require.NoError(t, err)
			}

			if tc.existingDockerRegistryDeployment != nil {
				_, err := createDeployment(actionContext.Context, k8sClient, tc.existingDockerRegistryDeployment)
				require.NoError(t, err)
			}

			err = action.Run(actionContext)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedReconcilerConfiguration, actionContext.Model.Configuration)
		})
	}

}

func createSecret(ctx context.Context, client kubernetes.Interface, secret *corev1.Secret) (*corev1.Secret, error) {
	return client.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func createDeployment(ctx context.Context, client kubernetes.Interface, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	return client.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
}
