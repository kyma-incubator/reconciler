package busolamigrator

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "busola-migrator"

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

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
		WithDependencies(istio.ReconcilerName).
		//register reconciler pre-action (executed BEFORE reconciliation happens)
		WithPreReconcileAction(&VirtSvcPreReconcilePatch{
			name:            "pre-action",
			virtSvcsToPatch: virtSvcs,
			suffix:          "-old",
			virtSvcClient:   virtSvcClient,
		})
}
