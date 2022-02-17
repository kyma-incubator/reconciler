package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterProcessingDuration(reconciliations reconciliation.Repository, reconcilerList []string, logger *zap.SugaredLogger) {
	processingDurationCollector := NewProcessingDurationCollector(reconciliations, reconcilerList, logger)
	prometheus.MustRegister(processingDurationCollector)

}

func RegisterWaitingAndNotReadyReconciliations(inventory cluster.Inventory, logger *zap.SugaredLogger) {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	prometheus.MustRegister(reconciliationWaitingCollector, reconciliationNotReadyCollector)
}

func RegisterOccupancy(occupancyRepo occupancy.Repository, reconcilerList []string, logger *zap.SugaredLogger) {
	prometheus.MustRegister(NewWorkerPoolOccupancyCollector(occupancyRepo, reconcilerList, logger))
}
