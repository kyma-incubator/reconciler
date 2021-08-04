package example

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "example"

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

	//configure reconciler
	reconciler.
		//list dependencies (these components have to be available before this component reconciler is able to run)
		WithDependencies("componentX", "componentY", "componentZ").
		//register reconciler pre-action (executed BEFORE reconciliation happens)
		WithPreReconcileAction(&CustomAction{
			name: "pre-action",
		}).
		//register reconciler action (custom reconciliation logic)
		WithReconcileAction(&CustomAction{
			name: "install-action",
		}).
		//register reconciler post-action (executed AFTER reconciliation happened)
		WithPostReconcileAction(&CustomAction{
			name: "post-action",
		})
}
