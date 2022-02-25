package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterProcessingDuration(reconciliations reconciliation.Repository, cfg *config.Config, logger *zap.SugaredLogger) {
	processingDurationCollector := NewProcessingDurationCollector(reconciliations, logger)
	prometheus.MustRegister(processingDurationCollector)

}

func RegisterWaitingAndNotReadyReconciliations(inventory cluster.Inventory, logger *zap.SugaredLogger) {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	prometheus.MustRegister(reconciliationWaitingCollector, reconciliationNotReadyCollector)
}

func RegisterOccupancy(occupancyRepo occupancy.Repository, cfg *config.Config, logger *zap.SugaredLogger) {
	prometheus.MustRegister(NewWorkerPoolOccupancyCollector(occupancyRepo, cfg, logger))
}
