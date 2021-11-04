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
		WithPreReconcileAction(&preInstallAction{
			&oryAction{step: "pre-install"},
		}).
		WithPostReconcileAction(&postInstallAction{
			&oryAction{step: "post-install"},
		})

	reconciler.
		WithPreDeleteAction(&preDeleteAction{
			&oryAction{step: "pre-delete"},
	})
}
