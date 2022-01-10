package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterAll(inventory cluster.Inventory, reconRepository reconciliation.Repository, logger *zap.SugaredLogger) {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	freeWorkersRatioCollector := NewFreeWorkersRatioCollector(reconRepository, logger)
	prometheus.MustRegister(reconciliationWaitingCollector, reconciliationNotReadyCollector, freeWorkersRatioCollector)
}
