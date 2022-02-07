package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"time"
)

// TODO: come up with reasonable prefixes
// label for metrics is prefix + operation name (component name of operation)
const prefixOperationLifetimeMothershipSuccessful = "0"
const prefixOperationLifetimeMothershipUnsuccessful = "1"
const prefixOperationProcessingTimeMothershipSuccessful = "2"
const prefixOperationProcessingTimeMothershipUnsuccessful = "3"
const prefixOperationProcessingTimeComponentSuccessful = "4"
const prefixOperationProcessingTimeComponentUnsuccessful = "5"

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
			prefixOperationLifetimeMothershipSuccessful,
			prefixOperationLifetimeMothershipUnsuccessful,
			prefixOperationProcessingTimeMothershipSuccessful,
			prefixOperationProcessingTimeMothershipUnsuccessful,
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
		for _, metric := range c.metricsList {
			m, err := c.reconciliationStatusGauge.GetMetricWithLabelValues(metric + component)
			if err != nil {
				c.logger.Errorf("unable to retrieve metric with label=%s: %s", component, err.Error())
				return
			}
			processingTime, err := c.getProcessingTime(component, metric)
			if err != nil {
				c.logger.Errorf(err.Error())
				continue
			}
			m.Set(processingTime.Seconds()) // TODO: Maybe smaller, but float64 here needed
		}
	}
	c.reconciliationStatusGauge.Collect(ch)

}

func (c *ProcessingTimeCollector) getProcessingTime(component, metric string) (time.Duration, error) {
	//TODO: Calulate metrics here
	switch metric {
	case prefixOperationLifetimeMothershipSuccessful:
		return c.reconRepo.GetMeanOperationLifetime(component, model.OperationStateDone)
	case prefixOperationLifetimeMothershipUnsuccessful:
		return c.reconRepo.GetMeanOperationLifetime(component, model.OperationStateError)
	case prefixOperationProcessingTimeMothershipSuccessful:
		//TODO
		return 0, nil
	case prefixOperationProcessingTimeMothershipUnsuccessful:
		//TODO
		return 0, nil
	case prefixOperationProcessingTimeComponentSuccessful:
		//TODO
		return 0, nil
	case prefixOperationProcessingTimeComponentUnsuccessful:
		//TODO
		return 0, nil
	}
	return 0, nil
}
