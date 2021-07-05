package metrics

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// ReconciliationNotReadyCollector provides number of clusters currently reconciling or in error/failed state:
// - reconciler_reconciliation_not_ready_total - total number of clusters currently reconciling or in error/failed state
type ReconciliationNotReadyCollector struct {
	inventory cluster.Inventory
	logger    *zap.Logger

	notReadyClustersDesc *prometheus.Desc
}

func NewReconciliationNotReadyCollector(inventory cluster.Inventory, logger *zap.Logger) *ReconciliationNotReadyCollector {
	return &ReconciliationNotReadyCollector{
		inventory: inventory,
		logger:    logger,
		notReadyClustersDesc: prometheus.NewDesc(prometheus.BuildFQName("", prometheusSubsystem, "reconciliation_not_ready_total"),
			"Total number of clusters currently reconciling or in error/failed state",
			[]string{},
			nil),
	}
}

func (c *ReconciliationNotReadyCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.notReadyClustersDesc
}

// Collect implements the prometheus.Collector interface.
func (c *ReconciliationNotReadyCollector) Collect(ch chan<- prometheus.Metric) {
	if c.inventory == nil {
		c.logger.Error("unable to register metric: inventory is nil")
		return
	}

	clusters, err := c.inventory.ClustersNotReady()
	if err != nil {
		c.logger.Error(err.Error())
		return
	}

	m, err := prometheus.NewConstMetric(c.notReadyClustersDesc, prometheus.GaugeValue, float64(len(clusters)))
	if err != nil {
		c.logger.Error(fmt.Sprintf("unable to register metric %s", err.Error()))
		return
	}

	ch <- m
}
