package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// WorkerPoolOccupancyCollector provides the ratio of free workers in the worker-pool:
// - mothership_free_workers - number of free workers in the worker-pool
type WorkerPoolOccupancyCollector struct {
	reconRepository reconciliation.Repository
	logger    *zap.SugaredLogger
	freeWorkersCntDesc *prometheus.Desc
}

func NewFreeWorkersRatioCollector(reconRepository reconciliation.Repository, logger *zap.SugaredLogger) *WorkerPoolOccupancyCollector {
	return &WorkerPoolOccupancyCollector{
		reconRepository: reconRepository,
		logger:    logger,
		freeWorkersCntDesc: prometheus.NewDesc(prometheus.BuildFQName("", prometheusSubsystem, "mothership_free_workers"),
			"Number of free workers in the worker-pool",
			[]string{},
			nil),
	}
}


func (c *WorkerPoolOccupancyCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.freeWorkersCntDesc
}

// Collect implements the prometheus.Collector interface.
func (c *WorkerPoolOccupancyCollector) Collect(ch chan<- prometheus.Metric) {
	if c.reconRepository == nil {
		c.logger.Error("unable to register metric: inventory is nil")
		return
	}

	workerPoolOccupancy, err := c.reconRepository.GetMeanWorkerPoolOccupancy()
	if err != nil {
		c.logger.Error(err.Error())
		return
	}

	m, err := prometheus.NewConstMetric(c.freeWorkersCntDesc, prometheus.GaugeValue, float64(workerPoolOccupancy))
	if err != nil {
		c.logger.Errorf("unable to register metric %s", err.Error())
		return
	}

	ch <- m
}
