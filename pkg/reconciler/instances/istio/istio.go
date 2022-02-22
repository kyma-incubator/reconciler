package istio

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/reset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	ReconcilerNameIstio              = "istio"
	ReconcilerNameIstioConfiguration = "istio-configuration"
)

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {

	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerNameIstio)
	reconcilerIstio, err := service.NewComponentReconciler(ReconcilerNameIstio)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerNameIstio, err)
	}

	gatherer := data.NewDefaultGatherer()
	matcher := pod.NewParentKindMatcher()
	provider := clientset.DefaultProvider{}
	action := reset.NewDefaultPodsResetAction(matcher)
	istioProxyReset := proxy.NewDefaultIstioProxyReset(gatherer, action)

	istioPerformerCreatorFn := istioPerformerCreator(istioProxyReset, &provider, ReconcilerNameIstio)
	reconcilerIstio.
		WithReconcileAction(NewReconcileAction(istioPerformerCreatorFn)).
		WithPostReconcileAction(NewMutatingWebhookPostAction(istioPerformerCreatorFn)).
		WithPostReconcileAction(NewProxyResetPostAction(istioPerformerCreatorFn)).
		WithDeleteAction(NewUninstallAction(istioPerformerCreatorFn))

	log.Debugf("Initializing component reconciler '%s'", ReconcilerNameIstioConfiguration)
	reconcilerIstioConfiguration, err := service.NewComponentReconciler(ReconcilerNameIstioConfiguration)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerNameIstioConfiguration, err)
	}

	istioConfigurationPerformerCreatorFn := istioPerformerCreator(istioProxyReset, &provider, ReconcilerNameIstioConfiguration)
	reconcilerIstioConfiguration.WithReconcileAction(NewReconcileIstioConfigurationAction(istioConfigurationPerformerCreatorFn)).
		WithDeleteAction(NewUninstallAction(istioConfigurationPerformerCreatorFn))

}
