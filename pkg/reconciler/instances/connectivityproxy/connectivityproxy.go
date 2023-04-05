package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	ReconcilerName = "connectivity-proxy"
)

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconcilerInstance, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	action := CustomAction{
		Name:   "action",
		Loader: &K8sLoader{},
		Commands: &CommandActions{
			install: service.NewInstall(log),
		},
	}
	reconcilerInstance.
		WithDeleteAction(&action).
		WithReconcileAction(&action)
}
