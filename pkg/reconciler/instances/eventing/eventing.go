package eventing

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/postaction"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/preaction"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	reconcilerName = "eventing"
)

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	logger := log.New()

	reconciler, err := service.NewComponentReconciler(reconcilerName)
	if err != nil {
		logger.With(log.KeyResult, log.ValueFail).With(log.KeyError, err).Fatal("Initialize component reconciler")
	}

	logger.With(log.KeyResult, log.ValueSuccess).Debug("Initialize component reconciler")
	reconciler.WithDependencies(istio.ReconcilerName).
		WithPreReconcileAction(preaction.New()).
		WithPostReconcileAction(postaction.New())
}
