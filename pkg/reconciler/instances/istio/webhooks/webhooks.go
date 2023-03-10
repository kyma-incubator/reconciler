package webhooks

import (
	"context"
	"fmt"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"go.uber.org/zap"
	"istio.io/istio/istioctl/pkg/tag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	retriesCount        = 5
	delayBetweenRetries = 5 * time.Second
)

var deactivatedLabel = map[string]string{
	"istio.io/deactivated": "never-match",
}

// DeleteConflictedDefaultTag deletes conflicted tagged MutatingWebhookConfiguration, if it exists and if the default revision MutatingWebhookConfiguration is not deactivated by istioctl.
func DeleteConflictedDefaultTag(ctx context.Context, provider clientset.Provider, kubeConfig string, logger *zap.SugaredLogger) error {
	kubeClient, err := provider.RetrieveFrom(kubeConfig, logger)
	if err != nil {
		logger.Error("Could not retrieve KubeClient from Kubeconfig!")
		return err
	}

	retryOpts := []retry.Option{
		retry.Delay(delayBetweenRetries),
		retry.Attempts(uint(retriesCount)),
		retry.DelayType(retry.FixedDelay),
	}

	err = retry.Do(func() error {
		webhooks, err := tag.GetWebhooksWithTag(ctx, kubeClient, tag.DefaultRevisionName)
		if err != nil {
			return err
		}

		if !isDefaultRevisionDeactivated(ctx, kubeClient) && len(webhooks) > 0 {
			for _, wh := range webhooks {
				err := kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(ctx, wh.Name, metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}

		return nil
	}, retryOpts...)

	if err != nil {
		return err
	}

	return nil
}

func isDefaultRevisionDeactivated(ctx context.Context, client kubernetes.Interface) bool {
	webhooks, err := tag.GetWebhooksWithRevision(ctx, client, tag.DefaultRevisionName)
	if err != nil {
		return true
	}
	if len(webhooks) == 0 {
		return true
	}

	webhook := webhooks[0]
	for i := range webhook.Webhooks {
		wh := webhook.Webhooks[i]
		if fmt.Sprint(wh.NamespaceSelector.MatchLabels) == fmt.Sprint(deactivatedLabel) || fmt.Sprint(wh.ObjectSelector) == fmt.Sprint(deactivatedLabel) {
			return true
		}
	}

	return false
}
