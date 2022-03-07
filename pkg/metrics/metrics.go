package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/features"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterAll(inventory cluster.Inventory, occupancyRepo occupancy.Repository, reconcilerList []string, logger *zap.SugaredLogger) {
	var collectors []prometheus.Collector
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	collectors = append(collectors, reconciliationWaitingCollector)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	collectors = append(collectors, reconciliationNotReadyCollector)

	if features.WorkerpoolOccupancyTrackingEnabled() {
		workerPoolOccupancyCollector := NewWorkerPoolOccupancyCollector(occupancyRepo, reconcilerList, logger)
		collectors = append(collectors, workerPoolOccupancyCollector)
	}
	prometheus.MustRegister(collectors...)
}
