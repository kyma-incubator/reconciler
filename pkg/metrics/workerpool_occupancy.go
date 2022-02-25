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
		labelValuesMap:      BuildLabelValuesMap(cfg),
		workerPoolOccupancyGaugeVec: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "worker_pool_occupancy_ratio",
			Help:      "Mean ratio of all running workers in all running worker-pools",
		}, []string{occupancyLabelName}),
	}
}

func (c *WorkerPoolOccupancyCollector) Describe(ch chan<- *prometheus.Desc) {
	c.workerPoolOccupancyGaugeVec.Describe(ch)
}

// Collect implements the prometheus.Collector interface.
func (c *WorkerPoolOccupancyCollector) Collect(ch chan<- prometheus.Metric) {

	for name, labelValue := range c.labelValuesMap {
		m, err := c.workerPoolOccupancyGaugeVec.GetMetricWithLabelValues(occupancyLabelName, labelValue)
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

func FormatPrometheusLabelNames(reconcilerNames []string) []string {
	promLabelNames := make([]string, 0, len(reconcilerNames))
	for _, name := range reconcilerNames {
		formattedName := strings.Replace(name, "-", "_", -1)
		promLabelNames = append(promLabelNames, formattedName)
	}
	return promLabelNames
}

func GetReconcilerList(cfg *config.Config) []string {
	reconcilerList := make([]string, 0, len(cfg.Scheduler.Reconcilers)+1)
	for reconciler := range cfg.Scheduler.Reconcilers {
		reconcilerList = append(reconcilerList, reconciler)
	}
	return append(reconcilerList, mothershipScalableServiceName)
}

func GetLabelValues(cfg *config.Config) []string {
	reconcilerList := GetReconcilerList(cfg)
	return FormatPrometheusLabelNames(reconcilerList)
}

func BuildLabelValuesMap(cfg *config.Config) map[string]string {
	labelValuesMap := make(map[string]string, len(cfg.Scheduler.Reconcilers)+1)
	for reconciler := range cfg.Scheduler.Reconcilers {
		labelValue := strings.Replace(reconciler, "-", "_", -1)
		labelValuesMap[reconciler] = labelValue
	}
	labelValuesMap[mothershipScalableServiceName] = mothershipScalableServiceName
	return labelValuesMap
}
