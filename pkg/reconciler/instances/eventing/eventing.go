package eventing

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	ReconcilerName = "eventing"
	actionName     = "pre-action"
)

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)
	if reconciler, err := service.NewComponentReconciler(ReconcilerName); err != nil {
		log.With(logKeyReconciler, ReconcilerName).With(logKeyResult, logValueFail).With(logKeyError, err).Error("Initialize component reconciler")
	} else {
		log.With(logKeyReconciler, ReconcilerName).With(logKeyResult, logValueSuccess).Debug("Initialize component reconciler")
		reconciler.WithDependencies(istio.ReconcilerName).WithPreReconcileAction(&preAction{name: actionName})
	}
}
