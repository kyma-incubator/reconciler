package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// WorkerPoolOccupancyCollector provides the mean ratio of running workers in the worker-pool(s):
// - worker_pool_occupancy - mean ratio of running workers in the worker-pool(s)
type WorkerPoolOccupancyCollector struct {
	occupancyRepository         occupancy.Repository
	logger                      *zap.SugaredLogger
	componentList               []string
	workerPoolOccupancyGaugeVec *prometheus.GaugeVec
}

func NewWorkerPoolOccupancyCollector(occupancyRepository occupancy.Repository, reconcilersList []string, logger *zap.SugaredLogger) *WorkerPoolOccupancyCollector {
	if occupancyRepository == nil {
		logger.Error("unable to register metric: repository is nil")
		return nil
	}
	return &WorkerPoolOccupancyCollector{
		occupancyRepository: occupancyRepository,
		logger:              logger,
		componentList:       reconcilersList,
		workerPoolOccupancyGaugeVec: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "worker_pool_occupancy",
			Help:      "Mean ratio of all running workers in all running worker-pools",
		}, reconcilersList),
	}
}

func (c *WorkerPoolOccupancyCollector) Describe(ch chan<- *prometheus.Desc) {
	c.workerPoolOccupancyGaugeVec.Describe(ch)
}

// Collect implements the prometheus.Collector interface.
func (c *WorkerPoolOccupancyCollector) Collect(ch chan<- prometheus.Metric) {

	for _, component := range c.componentList {
		m, err := c.workerPoolOccupancyGaugeVec.GetMetricWithLabelValues(component)
		if err != nil {
			c.logger.Errorf("unable to retrieve metric with label=%s: %s", component, err.Error())
			return
		}
		workerPoolOccupancy, err := c.occupancyRepository.GetMeanWorkerPoolOccupancyByComponent(component)
		if err != nil {
			c.logger.Error(err.Error())
			continue
		}
		m.Set(workerPoolOccupancy)
	}
	c.workerPoolOccupancyGaugeVec.Collect(ch)

}
