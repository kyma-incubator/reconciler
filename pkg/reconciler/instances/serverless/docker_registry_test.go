package serverless

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	existingUsername        = "some_username"
	existingPassword        = "some_password"
	existingRollme          = "tidD8"
	existingHTTPSSecret     = "some_http_secret"
	existingRegistryAddress = "some_registry_address"
	existingServerAddress   = "some_server_address"
)

func TestServerlessReconciliation(t *testing.T) {

	correctSecretData := map[string][]byte{
		"username":        []byte(existingUsername),
		"password":        []byte(existingPassword),
		"isInternal":      []byte("false"),
		"registryAddress": []byte(existingRegistryAddress),
		"serverAddress":   []byte(existingServerAddress),
	}

	expectedOverridesForCorrectSecretData := map[string]interface{}{
		"dockerRegistry.username":       existingUsername,
		"dockerRegistry.password":       existingPassword,
		"dockerRegistry.enableInternal": false,
	}

	correctAnnotations := map[string]string{"rollme": existingRollme}
	correctEnvs := []corev1.EnvVar{
		{Name: registryHTTPEnvKey, Value: existingHTTPSSecret},
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
			name: "Test true boolean override",
			existingSecret: fixedSecretWith(map[string][]byte{
				"isInternal": []byte("true"),
			}),
			expectedReconcilerConfiguration: map[string]interface{}{
				"dockerRegistry.enableInternal": true,
			},
		},
		{
			name: "Test false boolean override",
			existingSecret: fixedSecretWith(map[string][]byte{
				"isInternal": []byte("false"),
			}),
			expectedReconcilerConfiguration: map[string]interface{}{
				"dockerRegistry.enableInternal": false,
			},
		},
		{
			name:                            "Docker registry secret with empty strings found",
			existingSecret:                  fixedSecretWith(map[string][]byte{"username": []byte(""), "password": []byte("")}),
			expectedReconcilerConfiguration: map[string]interface{}{},
		},
		{
			name:                            "Secret with correct data found, no Deployment found",
			existingSecret:                  fixedSecretWith(correctSecretData),
			expectedReconcilerConfiguration: expectedOverridesForCorrectSecretData,
		},
		{
			name:                             "Both Secret and Deployment with correct data found",
			existingSecret:                   fixedSecretWith(correctSecretData),
			existingDockerRegistryDeployment: fixedDeploymentWith(correctAnnotations, correctEnvs),
			expectedReconcilerConfiguration: map[string]interface{}{
				"dockerRegistry.username":            existingUsername,
				"dockerRegistry.password":            existingPassword,
				"dockerRegistry.enableInternal":      false,
				"docker-registry.registryHTTPSecret": existingHTTPSSecret,
				"docker-registry.rollme":             existingRollme,
			},
		},
		{
			name:                             "Secret and Deployment ( empty data ) found",
			existingSecret:                   fixedSecretWith(correctSecretData),
			existingDockerRegistryDeployment: fixedDeploymentWith(map[string]string{}, []corev1.EnvVar{}),
			expectedReconcilerConfiguration:  expectedOverridesForCorrectSecretData,
		},
		{
			name:                             "Secret and Deployment ( nill data ) found",
			existingSecret:                   fixedSecretWith(correctSecretData),
			existingDockerRegistryDeployment: fixedDeploymentWith(nil, nil),
			expectedReconcilerConfiguration:  expectedOverridesForCorrectSecretData,
		},
		{
			name:           "Secret and Deployment ( empty strings ) found",
			existingSecret: fixedSecretWith(correctSecretData),
			existingDockerRegistryDeployment: fixedDeploymentWith(map[string]string{"rollme": ""}, []corev1.EnvVar{
				{Name: registryHTTPEnvKey, Value: ""},
			}),
			expectedReconcilerConfiguration: expectedOverridesForCorrectSecretData,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			var err error
			k8sClient, actionContext := setup()
			action := PreserveDockerRegistrySecret{}
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
			assert.Equal(t, tc.expectedReconcilerConfiguration, actionContext.Task.Configuration)
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
