package istio

import (
	"time"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/get"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-project/istio/operator/api/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var listOptions = []client.ListOption{client.InNamespace("kyma-system")}

const (
	istioFallbackVersion = "1.0.0"
	assetsURL            = "https://github.com/kyma-project/istio/releases/download"
)

const (
	retryDelay    = 10 * time.Second
	retryAttempts = 30
)

type MainReconcileAction struct{}

func NewIstioMainReconcileAction() *MainReconcileAction {
	return &MainReconcileAction{}
}

func (a *MainReconcileAction) Run(context *service.ActionContext) error {
	context.Logger.Debug("Reconcile action of Iistio triggered")

	cfg, err := buildConfig(context)
	if err != nil {
		return err
	}

	k8sClient, err := newClient(cfg)
	if err != nil {
		return err
	}

	istioVersion, err := get.IstioTagFromContext(context)
	if err != nil {
		context.Logger.Warnf("Could not get Istio Operator tag from Kyma chart, falling back to %s, err: %s", istioFallbackVersion, err)
		istioVersion = istioFallbackVersion
	}

	if err := installIstioModuleManifests(context, istioVersion, k8sClient); err != nil {
		return err
	}

	return installIstioCR(context, istioVersion, k8sClient)
}

func buildConfig(context *service.ActionContext) (*rest.Config, error) {
	return clientcmd.BuildConfigFromKubeconfigGetter("", func() (config *clientcmdapi.Config, e error) {
		return clientcmd.Load([]byte(context.KubeClient.Kubeconfig()))
	})
}

func newClient(cfg *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	return client.New(cfg, client.Options{
		Scheme: scheme,
	})
}

func installIstioModuleManifests(context *service.ActionContext, istioVersion string, k8sClient client.Client) error {
	managerManifests, err := get.IstioManagerManifest(assetsURL, istioVersion)
	if err != nil {
		return err
	}

	for _, manifest := range managerManifests {
		m := manifest
		spec := m.Object["spec"]
		_, err := controllerutil.CreateOrUpdate(context.Context, k8sClient, &m, func() error { m.Object["spec"] = spec; return nil })
		if err != nil {
			return err
		}
	}
	return nil
}

func installIstioCR(context *service.ActionContext, istioVersion string, k8sClient client.Client) error {
	crManifest, err := get.IstioCRManifest(assetsURL, istioVersion)
	if err != nil {
		return err
	}

	var istioList v1alpha1.IstioList
	err = k8sClient.List(context.Context, &istioList, listOptions...)
	if err != nil {
		return err
	}

	if len(istioList.Items) == 0 {
		context.Logger.Debug("Creating new Istio CR")
		err = k8sClient.Create(context.Context, &crManifest)
		if err != nil {
			return err
		}
	}

	return retry.Do(func() error {
		return checkIfIstioIsReady(context, k8sClient)
	}, retry.Attempts(retryAttempts), retry.Delay(retryDelay))
}

func getOldestCR(istioCRs *v1alpha1.IstioList) *v1alpha1.Istio {
	oldest := istioCRs.Items[0]
	for _, item := range istioCRs.Items {
		timestamp := &item.CreationTimestamp
		if !(oldest.CreationTimestamp.Before(timestamp)) {
			oldest = item
		}
	}
	return &oldest
}

func checkIfIstioIsReady(context *service.ActionContext, k8sClient client.Client) error {
	var istioList v1alpha1.IstioList
	err := k8sClient.List(context.Context, &istioList, listOptions...)
	if err != nil {
		return err
	}
	oldestCR := getOldestCR(&istioList)
	if oldestCR.Status.State == "Ready" {
		context.Logger.Info("Istio CR is in Ready state")
		return nil
	} else if oldestCR.Status.State == "Warning" {
		context.Logger.Warn("Istio CR is in Warning state")
		return nil
	} else if oldestCR.Status.State == "Error" {
		context.Logger.Error("Istio CR is in Error state")
		return nil
	}
	context.Logger.Debug("Waiting for Istio CR to get reconciled")
	return errors.New("Istio CR still reconciling")
}
