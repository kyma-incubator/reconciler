package istio

import (
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
	"time"
)

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
	context.Logger.Debug("Reconcile action of istio triggered")

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

	if err := installIstioCR(context, istioVersion, k8sClient); err != nil {
		return err
	}

	return nil
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
		spec := manifest.Object["spec"]
		_, err := controllerutil.CreateOrUpdate(context.Context, k8sClient, &manifest, func() error { manifest.Object["spec"] = spec; return nil })
		return err
	}
	return nil
}

func installIstioCR(context *service.ActionContext, istioVersion string, k8sClient client.Client) error {
	crManifest, err := get.IstioCRManifest(assetsURL, istioVersion)
	if err != nil {
		return err
	}

	var istioList v1alpha1.IstioList
	err = k8sClient.List(context.Context, &istioList)
	if err != nil {
		return err
	}

	if len(istioList.Items) == 0 {
		err = k8sClient.Create(context.Context, &crManifest)
		if err != nil {
			return err
		}
	}

	return retry.Do(func() error {
		return checkIfIstioIsReady(context, k8sClient)
	}, retry.Attempts(retryAttempts), retry.Delay(retryDelay))
}

func checkIfIstioIsReady(context *service.ActionContext, k8sClient client.Client) error {
	var istioList v1alpha1.IstioList
	err := k8sClient.List(context.Context, &istioList)
	if err != nil {
		return err
	}
	for _, item := range istioList.Items {
		if item.Status.State == "Ready" {
			return nil
		}
	}
	context.Logger.Debug("Waiting for Istio module to become ready")
	return errors.New("Istio module is not ready")
}
