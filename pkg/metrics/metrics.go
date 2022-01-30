package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterAll(inventory cluster.Inventory, workerRepository occupancy.Repository, reconcilerList []string, logger *zap.SugaredLogger) {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	workerPoolOccupancyCollector := NewWorkerPoolOccupancyCollector(workerRepository, reconcilerList, logger)
	prometheus.MustRegister(reconciliationWaitingCollector, reconciliationNotReadyCollector, workerPoolOccupancyCollector)
}
