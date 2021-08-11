package busola

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "busola-migrator"

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log, err := logger.NewLogger(true)
	if err != nil {
		panic(err)
	}

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	virtSvcClient := NewVirtSvcClient()
	virtSvcs := []VirtualSvcMeta{
		{
			Name:      "console-web",
			Namespace: "kyma-system",
		},
		{
			Name:      "dex-virtualservice",
			Namespace: "kyma-system",
		},
	}

	//configure reconciler
	reconciler.
		//list dependencies (these components have to be available before this component reconciler is able to run)
		WithDependencies("istio", "dex", "console").
		//register reconciler pre-action (executed BEFORE reconciliation happens)
		WithPreReconcileAction(&VirtualServicePreInstallPatch{
			name:            "pre-action",
			virtSvcsToPatch: virtSvcs,
			suffix:          "-old",
			virtSvcClient:   virtSvcClient,
		})
}
