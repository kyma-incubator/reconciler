package istio

import (
	"fmt"
	"os"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "istio"

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

	istioctlPath, ok := os.LookupEnv("ISTIOCTL")
	if !ok {
		istioctlPath = istioctl1_10_2
	}
	if !file.Exists(istioctlPath) {
		panic(fmt.Errorf("Reference istioctl '%s' not found", istioctlPath))
	}
	log.Debugf("Istioctl found: '%s'", istioctlPath)

	reconciler.WithPreReconcileAction(&ReconcileAction{istioctlPath})
}
