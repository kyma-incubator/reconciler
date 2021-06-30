package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterAll(inventory cluster.Inventory, logger *zap.Logger) {
	reconciliationWaitingCollector := NewReconciliationWaitingCollector(inventory, logger)
	prometheus.MustRegister(reconciliationWaitingCollector)
}
