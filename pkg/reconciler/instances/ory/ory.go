package ory

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	hydra "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/hydra"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/k8s"
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
		WithPreReconcileAction(&PreReconcileAction{
			&OryAction{Step: "pre-reconcile"},
		}).
		WithPostDeleteAction(&PostDeleteAction{
			&OryAction{Step: "post-delete"},
		}).
		WithPostReconcileAction(&PostReconcileAction{
			&OryAction{Step: "post-reconcile"}, hydra.NewDefaultHydraSyncer(k8s.NewDefaultRolloutHandler()),
			k8s.NewDefaultRolloutHandler(),
		})
}
