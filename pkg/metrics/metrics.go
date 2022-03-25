package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/features"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterProcessingDuration(reconciliations reconciliation.Repository, logger *zap.SugaredLogger) {
	if features.Enabled(features.ProcessingDurationMetric) {
		processingDurationCollector := NewProcessingDurationCollector(reconciliations, logger)
		prometheus.MustRegister(processingDurationCollector)
	}
}

func RegisterWaitingAndNotReadyReconciliations(inventory cluster.Inventory, logger *zap.SugaredLogger) {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	prometheus.MustRegister(reconciliationWaitingCollector, reconciliationNotReadyCollector)
}

func RegisterDbPool(connPool db.Connection, logger *zap.SugaredLogger) {
	dbPoolMetricsCollector := NewDbPoolCollector(connPool, logger)
	prometheus.MustRegister(dbPoolMetricsCollector)
}

func RegisterOccupancy(occupancyRepo occupancy.Repository, reconcilers map[string]config.ComponentReconciler, logger *zap.SugaredLogger) {
	if features.Enabled(features.WorkerpoolOccupancyTracking) {
		prometheus.MustRegister(NewWorkerPoolOccupancyCollector(occupancyRepo, reconcilers, logger))
	}
}
