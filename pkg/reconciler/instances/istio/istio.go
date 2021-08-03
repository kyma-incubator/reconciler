package istio

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func init() {
	log, err := logger.NewLogger(true)
	if err != nil {
		panic(err)
	}

	log.Debug("Initializing component reconciler 'istio'")
	reconciler, err := service.NewComponentReconciler("istio")
	if err != nil {
		log.Fatalf("Could not create component reconciler: %s", err)
	}

	reconciler.WithPreReconcileAction(&InstallAction{})
}
