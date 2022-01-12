package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// WorkerPoolOccupancyCollector provides the ratio of free workers in the worker-pool:
// - mothership_free_workers - number of free workers in the worker-pool
type WorkerPoolOccupancyCollector struct {
	occupancyRepository         occupancy.Repository
	logger                      *zap.SugaredLogger
	componentList               []string
	workerPoolOccupancyGaugeVec *prometheus.GaugeVec
}

func NewWorkerPoolOccupancyCollector(occupancyRepository occupancy.Repository, logger *zap.SugaredLogger) *WorkerPoolOccupancyCollector {
	if occupancyRepository == nil {
		logger.Error("unable to register metric: repository is nil")
		return nil
	}
	componentList, err := occupancyRepository.GetComponentList()
	if err != nil {
		logger.Error(err.Error())
		return nil
	}
	return &WorkerPoolOccupancyCollector{
		occupancyRepository: occupancyRepository,
		logger:              logger,
		componentList:       componentList,
		workerPoolOccupancyGaugeVec: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "worker_pool_occupancy",
			Help:      "Mean ratio of all running workers in all running worker-pools",
		}, componentList),
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
			return
		}
		m.Set(workerPoolOccupancy)
	}
	c.workerPoolOccupancyGaugeVec.Collect(ch)

}

