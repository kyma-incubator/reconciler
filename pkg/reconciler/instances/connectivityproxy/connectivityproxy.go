package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "connectivity-proxy"

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log, err := logger.NewLogger(false)
	if err != nil {
		panic(err)
	}

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	reconciler.
		WithPreReconcileAction(&CustomAction{
			name: "pre-action",
		}).
		WithReconcileAction(&CustomAction{
			name: "install-action",
		}).
		WithPostReconcileAction(&CustomAction{
			name: "post-action",
		})
}
