package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// TODO: come up with reasonable prefixes
// label for metrics is prefix + operation name (component name of operation)
const prefixOperationLifetimeMotherSuccessful = ""
const prefixOperationLifetimeMotherUnsuccessful = ""
const prefixOperationProcessingTimeMotherSuccessful = ""
const prefixOperationProcessingTimeMotherUnsuccessful = ""
const prefixOperationProcessingTimeComponentSuccessful = ""
const prefixOperationProcessingTimeComponentUnsuccessful = ""

// TODO: Describe

type ProcessingTimeCollector struct {
	reconciliationStatusGauge *prometheus.GaugeVec
	componentList             []string
	metricsList               []string
	reconRepo                 reconciliation.Repository
	logger                    *zap.SugaredLogger
}

func NewProcessingTimeCollector(reconciliations reconciliation.Repository, reconcilerList []string, logger *zap.SugaredLogger) *ProcessingTimeCollector {
	collector := &ProcessingTimeCollector{
		reconciliationStatusGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "processing_time",
			Help:      "Average processing time of operations", //TODO: better explanation
		}, []string{"runtime_id", "runtime_name"}),
		componentList: reconcilerList,
		metricsList: []string{
			prefixOperationLifetimeMotherSuccessful,
			prefixOperationLifetimeMotherUnsuccessful,
			prefixOperationProcessingTimeMotherSuccessful,
			prefixOperationProcessingTimeMotherUnsuccessful,
			prefixOperationProcessingTimeComponentSuccessful,
			prefixOperationProcessingTimeComponentUnsuccessful},
		reconRepo: reconciliations,
		logger:    logger,
	}
	prometheus.MustRegister(collector)
	return collector
}

func (c *ProcessingTimeCollector) Describe(ch chan<- *prometheus.Desc) {
	c.reconciliationStatusGauge.Describe(ch)
}

func (c *ProcessingTimeCollector) Collect(ch chan<- prometheus.Metric) {

	for _, component := range c.componentList {
		//TODO: calculate each processing metrics for each component
		for _, metric := range c.metricsList {
			m, err := c.reconciliationStatusGauge.GetMetricWithLabelValues(metric + component)
			if err != nil {
				c.logger.Errorf("unable to retrieve metric with label=%s: %s", component, err.Error())
				return
			}

			//TODO: get metrics here and set it afterwards

			m.Set(processingTime)
		}
	}
	c.reconciliationStatusGauge.Collect(ch)
}
