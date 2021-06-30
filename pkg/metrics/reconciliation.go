package metrics

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// ReconciliationWaitingCollector provides number of clusters waiting to be reconciled:
// - reconciler_reconciliation_waiting_total - total number of clusters waiting to be reconciled
type ReconciliationWaitingCollector struct {
	inventory cluster.Inventory
	logger    *zap.Logger

	waitingClustersDesc *prometheus.Desc
}

func NewReconciliationWaitingCollector(inventory cluster.Inventory, logger *zap.Logger) *ReconciliationWaitingCollector {
	return &ReconciliationWaitingCollector{
		inventory: inventory,
		logger:    logger,
		waitingClustersDesc: prometheus.NewDesc(prometheus.BuildFQName("", prometheusSubsystem, "reconciliation_waiting_total"),
			"Total number of clusters waiting to be reconciled",
			[]string{},
			nil),
	}
}

func (c *ReconciliationWaitingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.waitingClustersDesc
}

// Collect implements the prometheus.Collector interface.
func (c *ReconciliationWaitingCollector) Collect(ch chan<- prometheus.Metric) {
	clusters, err := c.inventory.ClustersToReconcile()
	if err != nil {
		c.logger.Error(err.Error())
		return
	}

	m, err := prometheus.NewConstMetric(c.waitingClustersDesc, prometheus.GaugeValue, float64(len(clusters)))
	if err != nil {
		c.logger.Error(fmt.Sprintf("unable to register metric %s", err.Error()))
		return
	}

	ch <- m
}
