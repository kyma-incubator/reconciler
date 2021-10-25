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

const (
	existingUsername    = "some_username"
	existingPassword    = "some_password"
	existingRollme      = "tidD8"
	existingHTTPSSecret = "some_http_secret"
)

func TestServerlessReconcilation(t *testing.T) {

	correctSecretData := map[string][]byte{
		"username": []byte(existingUsername),
		"password": []byte(existingPassword),
	}
	correctAnnotations := map[string]string{"rollme": existingRollme}
	correctEnvs := []corev1.EnvVar{
		{Name: registryHTTPSecretEnvKey, Value: existingHTTPSSecret},
	}

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
		{
			name:                            "Empty docker registry secret found",
			existingSecret:                  fixedSecretWith(map[string][]byte{}),
			expectedReconcilerConfiguration: map[string]interface{}{},
		},
		{
			name:                            "Docker registry secret with empty strings found",
			existingSecret:                  fixedSecretWith(map[string][]byte{"username": []byte(""), "password": []byte("")}),
			expectedReconcilerConfiguration: map[string]interface{}{},
		},
		{
			name:           "Secret with correct data found, no Deployment found",
			existingSecret: fixedSecretWith(correctSecretData),
			expectedReconcilerConfiguration: map[string]interface{}{
				"dockerRegistry.username": existingUsername,
				"dockerRegistry.password": existingPassword,
			},
		},
		{
			name:                             "Both Secret and Deployment with correct data found",
			existingSecret:                   fixedSecretWith(correctSecretData),
			existingDockerRegistryDeployment: fixedDeploymentWith(correctAnnotations, correctEnvs),
			expectedReconcilerConfiguration: map[string]interface{}{
				"dockerRegistry.username":            existingUsername,
				"dockerRegistry.password":            existingPassword,
				"docker-registry.registryHTTPSecret": existingHTTPSSecret,
				"docker-registry.rollme":             existingRollme,
			},
		},
		{
			name:                             "Secret and Deployment ( empty data ) found",
			existingSecret:                   fixedSecretWith(correctSecretData),
			existingDockerRegistryDeployment: fixedDeploymentWith(map[string]string{}, []corev1.EnvVar{}),
			expectedReconcilerConfiguration: map[string]interface{}{
				"dockerRegistry.username": existingUsername,
				"dockerRegistry.password": existingPassword,
			},
		},
		{
			name:                             "Secret and Deployment ( nill data ) found",
			existingSecret:                   fixedSecretWith(correctSecretData),
			existingDockerRegistryDeployment: fixedDeploymentWith(nil, nil),
			expectedReconcilerConfiguration: map[string]interface{}{
				"dockerRegistry.username": existingUsername,
				"dockerRegistry.password": existingPassword,
			},
		},
		{
			name:           "Secret and Deployment ( empty strings ) found",
			existingSecret: fixedSecretWith(correctSecretData),
			existingDockerRegistryDeployment: fixedDeploymentWith(map[string]string{"rollme": ""}, []corev1.EnvVar{
				{Name: registryHTTPSecretEnvKey, Value: ""},
			}),
			expectedReconcilerConfiguration: map[string]interface{}{
				"dockerRegistry.username": existingUsername,
				"dockerRegistry.password": existingPassword,
			},
		},
	}

	setup := func() (kubernetes.Interface, ReconcileCustomAction, *service.ActionContext) {
		k8sClient := fake.NewSimpleClientset()

		action := ReconcileCustomAction{}
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

			//GIVEN
			if tc.existingSecret != nil {
				_, err := createSecret(actionContext.Context, k8sClient, tc.existingSecret)
				require.NoError(t, err)
			}

			if tc.existingDockerRegistryDeployment != nil {
				_, err := createDeployment(actionContext.Context, k8sClient, tc.existingDockerRegistryDeployment)
				require.NoError(t, err)
			}

			//WHEN
			err = action.Run(actionContext)

			//THEN
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

func fixedSecretWith(data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverlessSecretName,
			Namespace: serverlessNamespace,
		},
		Data: data,
	}
}

func fixedDeploymentWith(annotations map[string]string, envs []corev1.EnvVar) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverlessDockerRegistryDeploymentName,
			Namespace: serverlessNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "irrelevant",
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "irrelevant",
							Image: "irrelevant",
							Env:   envs,
						},
					},
				},
			},
		},
	}
}
