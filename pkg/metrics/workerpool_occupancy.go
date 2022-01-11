package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// WorkerPoolOccupancyCollector provides the ratio of free workers in the worker-pool:
// - mothership_free_workers - number of free workers in the worker-pool
type WorkerPoolOccupancyCollector struct {
	workerRepository occupancy.Repository
	logger           *zap.SugaredLogger
	freeWorkersCntDesc *prometheus.Desc
}

func NewFreeWorkersRatioCollector(workerRepository occupancy.Repository, logger *zap.SugaredLogger) *WorkerPoolOccupancyCollector {
	return &WorkerPoolOccupancyCollector{
		workerRepository: workerRepository,
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
	if c.workerRepository == nil {
		c.logger.Error("unable to register metric: inventory is nil")
		return
	}

	workerPoolOccupancy, err := c.workerRepository.GetMeanWorkerPoolOccupancy()
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
