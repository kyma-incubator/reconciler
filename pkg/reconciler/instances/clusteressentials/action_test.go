package clusteressentials

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mock "github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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
	existingCrt    = "sample_cert_value"
	existingKey    = "sample_cert_key"
	existingCaCert = "sample_CA_cert"
)

func TestServerlessReconcilation(t *testing.T) {

	correctSecretData := map[string][]byte{
		"tls.crt": []byte(existingCrt),
		"tls.key": []byte(existingKey),
	}

	testCases := []struct {
		name                            string
		existingSecret                  *corev1.Secret
		existingMutatingWebhookConfig   *admissionregistrationv1.MutatingWebhookConfiguration
		expectedReconcilerConfiguration map[string]interface{}
	}{
		{
			name:                            "No webhook cert secret found",
			expectedReconcilerConfiguration: map[string]interface{}{},
		},
		{
			name:           "webhook cert secret found",
			existingSecret: fixedSecretWith(correctSecretData),
			expectedReconcilerConfiguration: map[string]interface{}{
				"pod-preset.cert": existingCrt,
				"pod-preset.key":  existingKey,
			},
		},
		{
			name:                            "mutating webhook config found but webhook name not match",
			existingMutatingWebhookConfig:   fixedMutatingWebhookConfigWith("foo", []byte(existingCaCert)),
			expectedReconcilerConfiguration: map[string]interface{}{},
		},
		{
			name:                          "mutating webhook config found",
			existingMutatingWebhookConfig: fixedMutatingWebhookConfigWith(webhookname, []byte(existingCaCert)),
			expectedReconcilerConfiguration: map[string]interface{}{
				"pod-preset.caCert": existingCaCert,
			},
		},
		{
			name:                          "mutating webhook config and webhook cert secret found",
			existingSecret:                fixedSecretWith(correctSecretData),
			existingMutatingWebhookConfig: fixedMutatingWebhookConfigWith(webhookname, []byte(existingCaCert)),
			expectedReconcilerConfiguration: map[string]interface{}{
				"pod-preset.caCert": existingCaCert,
				"pod-preset.cert":   existingCrt,
				"pod-preset.key":    existingKey,
			},
		},
		{
			name:                            "mutating webhook config with empty clientData found",
			existingMutatingWebhookConfig:   fixedMutatingWebhookConfigWithClientConfig(webhookname, admissionregistrationv1.WebhookClientConfig{}),
			expectedReconcilerConfiguration: map[string]interface{}{},
		},
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

			if tc.existingMutatingWebhookConfig != nil {
				_, err := createMutatingWebhookConfiguration(actionContext.Context, k8sClient, tc.existingMutatingWebhookConfig)
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

func createMutatingWebhookConfiguration(ctx context.Context, client kubernetes.Interface, config *admissionregistrationv1.MutatingWebhookConfiguration) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	return client.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(ctx, config, metav1.CreateOptions{})
}

func fixedSecretWith(data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookCertSecretName,
			Namespace: webhookCertSecretNamespace,
		},
		Data: data,
	}
}

func fixedMutatingWebhookConfigWith(name string, ca []byte) *admissionregistrationv1.MutatingWebhookConfiguration {
	return fixedMutatingWebhookConfigWithClientConfig(name, admissionregistrationv1.WebhookClientConfig{
		CABundle: ca,
	})

}

func fixedMutatingWebhookConfigWithClientConfig(name string, clientConfig admissionregistrationv1.WebhookClientConfig) *admissionregistrationv1.MutatingWebhookConfiguration {
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: mutatingWebhookConfigName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			{
				Name:         name,
				ClientConfig: clientConfig,
			},
		},
	}
}

func setup() (kubernetes.Interface, CustomAction, *service.ActionContext) {
	k8sClient := fake.NewSimpleClientset()

	action := CustomAction{}
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
		Task:          &reconciler.Task{Version: "test", Configuration: configuration},
	}
	return k8sClient, action, actionContext
}
