package test

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/hydra"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/k8s"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "ory"

func TestOryIntegration(t *testing.T) {
	//OryIntegrationTest(t)
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	reconciler.
		WithPreReconcileAction(&ory.PreReconcileAction{
			&ory.OryAction{Step: "pre-reconcile"},
		}).
		WithPostDeleteAction(&ory.PostDeleteAction{
			&ory.OryAction{Step: "post-delete"},
		}).
		WithPostReconcileAction(&ory.PostReconcileAction{
			&ory.OryAction{Step: "post-reconcile"}, hydra.NewDefaultHydraSyncer(k8s.NewDefaultRolloutHandler()), k8s.NewDefaultRolloutHandler(),
		})

}
