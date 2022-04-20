package scmigration

import (
	"os"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "sc-migration"

//nolint:gochecknoinits
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	skipSafeCheck := os.Getenv("SKIP_SAFE_DELETION_BROKER_CHECK")
	reconciler.WithReconcileAction(&reconcileAction{skipSafeCheck: skipSafeCheck == "true"})
}
