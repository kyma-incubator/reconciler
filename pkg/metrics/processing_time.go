package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// label for metrics is prefix + operation name (component name of operation) + unit suffix
// stick to prometheus naming conventions
const prefixOperationLifetimeMothershipSuccessful = "operation_lifetime_mothership_successful_"
const prefixOperationLifetimeMothershipUnsuccessful = "operation_lifetime_mothership_unsuccessful_"
const prefixOperationProcessingDurationMothershipSuccessful = "operation_processing_duration_mothership_successful_"
const prefixOperationProcessingDurationMothershipUnsuccessful = "operation_processing_duration_mothership_unsuccessful_"
const prefixOperationProcessingDurationComponentSuccessful = "operation_processing_duration_reconciler_successful_"
const prefixOperationProcessingDurationComponentUnsuccessful = "operation_processing_duration_reconciler_unsuccessful_"

const suffixUnit = "milliseconds"

// ProcessingDurationCollector provides average duration of last 500 operations in error and done state:
// - operation_lifetime_mothership_successful_<component-name>_milliseconds - avg. lifetime time of a component-operation in the mothership reconciler (created and successfully finished)
// - operation_lifetime_mothership_unsuccessful_<component-name>_milliseconds - avg. lifetime time of a component-operation in the mothership reconciler (created and non-successfully finished)
// - operation_processing_duration_mothership_successful_<component-name>_milliseconds - the avg. processing time by the mothership reconciler (picked up and finished by worker in mothership-reconciler if operation was successful)
// - operation_processing_duration_mothership_unsuccessful_<component-name>_milliseconds - the avg. processing time by the mothership reconciler (picked up and finished by worker in mothership-reconciler if operation was non-successful)
// - operation_processing_duration_reconciler_successful_<component-name>_milliseconds - avg. processing time by the component reconciler (rendered and finished to be deployed in K8s successfully)
// - operation_processing_duration_reconciler_unsuccessful_<component-name>_milliseconds - avg. processing time by the component reconciler (rendered and finished to be deployed in K8s non-successfully)
type ProcessingDurationCollector struct {
	processingDurationHistogram *prometheus.HistogramVec
	metricsList                 []string
	reconRepo                   reconciliation.Repository
	logger                      *zap.SugaredLogger
}

func NewProcessingDurationCollector(reconciliations reconciliation.Repository, logger *zap.SugaredLogger) *ProcessingDurationCollector {
	return &ProcessingDurationCollector{
		processingDurationHistogram: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: prometheusSubsystem,
			Name:      "processing_time",
			Help:      "Processing time of operations",
			Buckets:   prometheus.ExponentialBuckets(100, 2, 15),
		}, []string{"component", "metric"}),
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
}

func (c *ProcessingDurationCollector) Describe(ch chan<- *prometheus.Desc) {
	c.processingDurationHistogram.Describe(ch)
}

func (c *ProcessingDurationCollector) Collect(ch chan<- prometheus.Metric) {

	componentList, err := c.reconRepo.GetAllComponents()
	if err != nil {
		c.logger.Warnf("Could not receive componentList from db: %s", err)
	}

	for _, component := range componentList {
		for _, metric := range c.metricsList {
			m, err := c.processingDurationHistogram.GetMetricWithLabelValues(component, metric+suffixUnit)
			if err != nil {
				c.logger.Errorf("processingDurationCollector: unable to retrieve metric with label=%s: %s", component, err.Error())
				return
			}
			processingDuration, err := c.getProcessingDuration(component, metric)
			if err != nil {
				c.logger.Errorf("Error getting ProcessingDuration: %s", err.Error())
				continue
			}
			m.Observe(float64(processingDuration))
		}
	}
	c.processingDurationHistogram.Collect(ch)
}

func (c *ProcessingDurationCollector) getProcessingDuration(component, metric string) (int64, error) {
	switch metric {
	case prefixOperationLifetimeMothershipSuccessful:
		return c.reconRepo.GetMothershipOperationProcessingDuration(component, model.OperationStateDone, reconciliation.Created)
	case prefixOperationLifetimeMothershipUnsuccessful:
		return c.reconRepo.GetMothershipOperationProcessingDuration(component, model.OperationStateError, reconciliation.Created)
	case prefixOperationProcessingDurationMothershipSuccessful:
		return c.reconRepo.GetMothershipOperationProcessingDuration(component, model.OperationStateDone, reconciliation.PickedUp)
	case prefixOperationProcessingDurationMothershipUnsuccessful:
		return c.reconRepo.GetMothershipOperationProcessingDuration(component, model.OperationStateError, reconciliation.PickedUp)
	case prefixOperationProcessingDurationComponentSuccessful:
		return c.reconRepo.GetComponentOperationProcessingDuration(component, model.OperationStateDone)
	case prefixOperationProcessingDurationComponentUnsuccessful:
		return c.reconRepo.GetComponentOperationProcessingDuration(component, model.OperationStateError)
	}
	return 0, nil
}
