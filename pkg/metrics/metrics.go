package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/features"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterAll(inventory cluster.Inventory, reconciliations reconciliation.Repository, occupancyRepo occupancy.Repository, reconcilerList []string, logger *zap.SugaredLogger, occupancyTracking bool) {
	var collectors []prometheus.Collector
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	collectors = append(collectors, reconciliationWaitingCollector)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	collectors = append(collectors, reconciliationNotReadyCollector)
	if features.ProcessingDurationMetricsEnabled() {
		processingDurationCollector := NewProcessingDurationCollector(reconciliations, logger)
		collectors = append(collectors, processingDurationCollector)
	}
	if occupancyTracking {
		workerPoolOccupancyCollector := NewWorkerPoolOccupancyCollector(occupancyRepo, reconcilerList, logger)
		collectors = append(collectors, workerPoolOccupancyCollector)
	}
	prometheus.MustRegister(collectors...)
}
