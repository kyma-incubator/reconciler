package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterAll(inventory cluster.Inventory, reconciliations reconciliation.Repository, occupancyRepo occupancy.Repository, reconcilerList []string, logger *zap.SugaredLogger, occupancyTracking bool) {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	processingDurationCollector := NewProcessingDurationCollector(reconciliations, logger)
	if !occupancyTracking {
		prometheus.MustRegister(reconciliationWaitingCollector, reconciliationNotReadyCollector, processingDurationCollector)
	} else {
		workerPoolOccupancyCollector := NewWorkerPoolOccupancyCollector(occupancyRepo, reconcilerList, logger)
		prometheus.MustRegister(reconciliationWaitingCollector, reconciliationNotReadyCollector, processingDurationCollector, workerPoolOccupancyCollector)
	}
}
