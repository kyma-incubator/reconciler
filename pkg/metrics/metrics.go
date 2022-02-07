package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterAll(inventory cluster.Inventory, reconciliations reconciliation.Repository, reconcilerList []string, logger *zap.SugaredLogger) {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	reconciliationNotReadyCollector := NewReconciliationNotReadyCollector(inventory, logger)
	processingTimeCollector := NewProcessingTimeCollector(reconciliations, reconcilerList, logger)
	prometheus.MustRegister(reconciliationWaitingCollector, reconciliationNotReadyCollector, processingTimeCollector)
}
