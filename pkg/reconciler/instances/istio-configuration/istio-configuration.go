package istio_configuration

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio-configuration/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio-configuration/clientset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio-configuration/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio-configuration/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio-configuration/reset/pod"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio-configuration/reset/pod/reset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio-configuration/reset/proxy"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "istio-configuration"

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	commander := istioctl.DefaultCommander{}
	gatherer := data.NewDefaultGatherer()
	matcher := pod.NewParentKindMatcher()
	provider := clientset.DefaultProvider{}
	action := reset.NewDefaultPodsResetAction(matcher)
	istioProxyReset := proxy.NewDefaultIstioProxyReset(gatherer, action)
	performer := actions.NewDefaultIstioPerformer(&commander, istioProxyReset, &provider)
	reconciler.WithReconcileAction(&ReconcileAction{
		performer: performer,
	})
}
