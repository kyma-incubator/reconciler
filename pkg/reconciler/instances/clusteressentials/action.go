package clusteressentials

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/utils"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mutatingWebhookConfigName  = "cluster-essentials-pod-preset-webhook"
	webhookname                = "podpresets.settings.svcat.k8s.io"
	webhookCertSecretName      = "cluster-essentials-pod-preset-webhook-cert"
	webhookCertSecretNamespace = "kyma-system"
)

type CustomAction struct {
	name string
}

func (a *CustomAction) Run(context *service.ActionContext) error {
	logger := context.Logger
	k8sClient, err := context.KubeClient.Clientset()
	if err != nil {
		context.Logger.Errorf("Failed to retrieve native Kubernetes GO client")
	}

	logger.Infof("Action '%s' executed (passed version was '%s')", a.name, context.Task.Version)

	mutatingWebhookConfiguration, err := k8sClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.Context, mutatingWebhookConfigName, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("Error while fetching existing mutatingWebhookConfigurations [%s]... Certificates will be re-generated", err.Error())
	} else if mutatingWebhookConfiguration != nil {
		webhooks := mutatingWebhookConfiguration.Webhooks
		for _, w := range webhooks {
			if w.Name == webhookname && w.ClientConfig.CABundle != nil {
				logger.Infof("MutatingWebhookConfiguration [%s/%s] found. Attempting to reusing existing CA bundle", mutatingWebhookConfigName, webhookname)
				context.Task.Configuration["pod-preset.caCert"] = string(w.ClientConfig.CABundle)
			}
		}
	}

	secret, err := k8sClient.CoreV1().Secrets(webhookCertSecretNamespace).Get(context.Context, webhookCertSecretName, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("Error while fetching secret [%s]... Certificate will be re-generated", err.Error())
	} else if secret != nil {
		utils.SetOverrideFromSecret(logger, secret, context.Task.Configuration, "tls.crt", "pod-preset.cert")
		utils.SetOverrideFromSecret(logger, secret, context.Task.Configuration, "tls.key", "pod-preset.key")
	}

	return service.NewInstall(context.Logger).Invoke(context.Context, context.ChartProvider, context.Task, context.KubeClient)
}
