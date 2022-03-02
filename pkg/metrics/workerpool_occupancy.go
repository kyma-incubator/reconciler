package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"strings"
)

const (
	mothershipScalableServiceName = "mothership"
	occupancyLabelName            = "component"
)

// WorkerPoolOccupancyCollector provides the mean ratio of running workers in the worker-pool(s):
// - worker_pool_occupancy - mean ratio of running workers in the worker-pool(s)
type WorkerPoolOccupancyCollector struct {
	occupancyRepository         occupancy.Repository
	logger                      *zap.SugaredLogger
	labelValuesMap              map[string]string
	workerPoolOccupancyGaugeVec *prometheus.GaugeVec
}

func NewWorkerPoolOccupancyCollector(occupancyRepository occupancy.Repository, cfg *config.Config, logger *zap.SugaredLogger) *WorkerPoolOccupancyCollector {
	if occupancyRepository == nil {
		logger.Error("unable to register metric: repository is nil")
		return nil
	}
	return &WorkerPoolOccupancyCollector{
		occupancyRepository: occupancyRepository,
		logger:              logger,
		labelValuesMap:      buildLabelValuesMap(cfg),
		workerPoolOccupancyGaugeVec: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "worker_pool_occupancy_ratio",
			Help:      "Mean ratio of all running workers in all running worker-pools for every reconciler",
		}, []string{occupancyLabelName}),
	}
}

func (c *WorkerPoolOccupancyCollector) Describe(ch chan<- *prometheus.Desc) {
	c.workerPoolOccupancyGaugeVec.Describe(ch)
}

// Collect implements the prometheus.Collector interface.
func (c *WorkerPoolOccupancyCollector) Collect(ch chan<- prometheus.Metric) {

	for name, labelValue := range c.labelValuesMap {
		m, err := c.workerPoolOccupancyGaugeVec.GetMetricWithLabelValues(labelValue)
		if err != nil {
			c.logger.Errorf("workerPoolOccupancyCollector: unable to retrieve metric for component=%s: %s", name, err)
			return
		}
		workerPoolOccupancy, err := c.occupancyRepository.GetMeanWorkerPoolOccupancyByComponent(name)
		if err != nil {
			c.logger.Error(err.Error())
			continue
		}
		m.Set(workerPoolOccupancy)
	}
	c.workerPoolOccupancyGaugeVec.Collect(ch)
}

func buildLabelValuesMap(cfg *config.Config) map[string]string {
	labelValuesMap := make(map[string]string, len(cfg.Scheduler.Reconcilers)+1)
	for reconciler := range cfg.Scheduler.Reconcilers {
		labelValue := strings.Replace(reconciler, "-", "_", -1)
		labelValuesMap[reconciler] = labelValue
	}
	labelValuesMap[mothershipScalableServiceName] = mothershipScalableServiceName
	return labelValuesMap
}
