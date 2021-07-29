package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// ReconciliationWaitingCollector provides number of clusters waiting to be reconciled:
// - reconciler_reconciliation_waiting_total - total number of clusters waiting to be reconciled
type ReconciliationWaitingCollector struct {
	inventory cluster.Inventory
	logger    *zap.SugaredLogger

	waitingClustersDesc *prometheus.Desc
}

func NewReconciliationWaitingCollector(inventory cluster.Inventory, logger *zap.SugaredLogger) *ReconciliationWaitingCollector {
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
	if c.inventory == nil {
		c.logger.Error("unable to register metric: inventory is nil")
		return
	}

	clusters, err := c.inventory.ClustersToReconcile(0)
	if err != nil {
		c.logger.Error(err.Error())
		return
	}

	m, err := prometheus.NewConstMetric(c.waitingClustersDesc, prometheus.GaugeValue, float64(len(clusters)))
	if err != nil {
		c.logger.Errorf("unable to register metric %s", err.Error())
		return
	}

	ch <- m
}
