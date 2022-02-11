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
const prefixOperationProcessingDurationMothershipSuccessful = "2"
const prefixOperationProcessingDurationMothershipUnsuccessful = "3"
const prefixOperationProcessingDurationComponentSuccessful = "4"
const prefixOperationProcessingDurationComponentUnsuccessful = "5"

// TODO: Describe

type ProcessingDurationCollector struct {
	reconciliationStatusGauge *prometheus.GaugeVec
	componentList             []string
	metricsList               []string
	reconRepo                 reconciliation.Repository
	logger                    *zap.SugaredLogger
}

func NewProcessingDurationCollector(reconciliations reconciliation.Repository, reconcilerList []string, logger *zap.SugaredLogger) *ProcessingDurationCollector {
	collector := &ProcessingDurationCollector{
		reconciliationStatusGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "processing_time",
			Help:      "Average processing time of operations", //TODO: better explanation
		}, []string{"runtime_id", "runtime_name"}),
		componentList: reconcilerList,
		metricsList: []string{
			prefixOperationLifetimeMothershipSuccessful,
			prefixOperationLifetimeMothershipUnsuccessful,
			prefixOperationProcessingDurationMothershipSuccessful,
			prefixOperationProcessingDurationMothershipUnsuccessful,
			prefixOperationProcessingDurationComponentSuccessful,
			prefixOperationProcessingDurationComponentUnsuccessful},
		reconRepo: reconciliations,
		logger:    logger,
	}
	prometheus.MustRegister(collector)
	return collector
}

func (c *ProcessingDurationCollector) Describe(ch chan<- *prometheus.Desc) {
	c.reconciliationStatusGauge.Describe(ch)
}

func (c *ProcessingDurationCollector) Collect(ch chan<- prometheus.Metric) {

	for _, component := range c.componentList {
		for _, metric := range c.metricsList {
			m, err := c.reconciliationStatusGauge.GetMetricWithLabelValues(metric + component)
			if err != nil {
				c.logger.Errorf("unable to retrieve metric with label=%s: %s", component, err.Error())
				return
			}
			processingDuration, err := c.getProcessingDuration(component, metric)
			if err != nil {
				c.logger.Errorf(err.Error())
				continue
			}
			m.Set(processingDuration.Seconds()) // TODO: Maybe smaller, but float64 here needed
		}
	}
	c.reconciliationStatusGauge.Collect(ch)

}

func (c *ProcessingDurationCollector) getProcessingDuration(component, metric string) (time.Duration, error) {
	//TODO: Calulate metrics here
	switch metric {
	case prefixOperationLifetimeMothershipSuccessful:
		return c.reconRepo.GetMeanMothershipOperationProcessingDuration(component, model.OperationStateDone, reconciliation.Created)
	case prefixOperationLifetimeMothershipUnsuccessful:
		return c.reconRepo.GetMeanMothershipOperationProcessingDuration(component, model.OperationStateError, reconciliation.Created)
	case prefixOperationProcessingDurationMothershipSuccessful:
		return c.reconRepo.GetMeanMothershipOperationProcessingDuration(component, model.OperationStateDone, reconciliation.PickedUp)
	case prefixOperationProcessingDurationMothershipUnsuccessful:
		return c.reconRepo.GetMeanMothershipOperationProcessingDuration(component, model.OperationStateError, reconciliation.PickedUp)
	case prefixOperationProcessingDurationComponentSuccessful:
		//TODO
		return 0, nil
	case prefixOperationProcessingDurationComponentUnsuccessful:
		//TODO
		return 0, nil
	}
	return 0, nil
}
