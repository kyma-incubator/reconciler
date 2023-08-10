package istio

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	ReconcilerNameIstio = "istio"
)

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {

	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerNameIstio)
	reconcilerIstio, err := service.NewComponentReconciler(ReconcilerNameIstio)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerNameIstio, err)
	}

	reconcilerIstio.WithReconcileAction(NewIstioMainReconcileAction())
}
