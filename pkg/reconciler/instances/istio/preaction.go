package istio

import (
	"context"
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	istioMutatingWebhook = "istio-sidecar-injector"
	istioAutoInjectorName = "auto.sidecar-injector.istio.io"
)

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type PreReconcileAction struct {}

func (a *PreReconcileAction) Run(context *service.ActionContext) error {
	client, err := context.KubeClient.Clientset()
	if err != nil {
		return err
	}

	context.Logger.Infof("Checking if %s webhook is present on the cluster...", istioAutoInjectorName)

	webhookConf, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.Context, istioMutatingWebhook, metav1.GetOptions{})
	if err != nil {
		return err
	}

	autoInjectorWebhookExists := false
	for _, webhook := range webhookConf.Webhooks {
		if webhook.Name == istioAutoInjectorName {
			autoInjectorWebhookExists = true
		}
	}

	if autoInjectorWebhookExists {
		context.Logger.Infof("%s webhook is present on the cluster - labeling existing namespaces to preserve enabled-by-default sidecar injection policy...", istioAutoInjectorName)

		err = labelNamespacesWithIstioInjectionWithout(getExcludedNamespaces(), client, context.Context, context.Logger)
		if err != nil {
			return err
		}
	}

	return nil
}

func labelNamespacesWithIstioInjectionWithout(excluded []string, client kubernetes.Interface, context context.Context, logger *zap.SugaredLogger) error {
	allNamespaces, err := client.CoreV1().Namespaces().List(context, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, namespace := range allNamespaces.Items {
		isExcluded := false
		for _, ex := range excluded {
			if ex == namespace.Name {
				isExcluded = true
			}
		}

		if !isExcluded {
			payload := []patchStringValue{{
				Op:    "replace",
				Path:  "/metadata/labels/istio-injection",
				Value: "enabled",
			}}

			payloadBytes, _ := json.Marshal(payload)
			_, err = client.CoreV1().Namespaces().Patch(context, namespace.Name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
			if err != nil {
				return err
			}

			logger.Infof("Successfully labeled namespace: %s", namespace.Name)
		}
	}

	return nil
}

func getExcludedNamespaces() []string {
	return []string{"istio-system", "kyma-system", "kube-system", "kube-node-lease", "kube-public"}
}