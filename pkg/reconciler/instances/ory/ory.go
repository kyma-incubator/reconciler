package ory

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "ory"

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	reconciler.
		WithPreReconcileAction(&preReconcileAction{
			&oryAction{step: "pre-install"},
		}).
		WithPostReconcileAction(&postReconcileAction{
			&oryAction{step: "post-install"},
		}).
		WithPreDeleteAction(&preDeleteAction{
			&oryAction{step: "pre-delete"},
		})
}
