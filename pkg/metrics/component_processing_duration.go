package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"time"
)

type ComponentProcessingDurationMetric struct {
	Collector *prometheus.HistogramVec
	logger    *zap.SugaredLogger
}

func NewComponentProcessingDurationMetric(logger *zap.SugaredLogger) *ComponentProcessingDurationMetric {
	const startBucketWithMicrosecond = 1e6
	return &ComponentProcessingDurationMetric{
		Collector: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: prometheusSubsystem,
			Name:      "processing_time",
			Help:      "Processing time of operations",
			Buckets:   prometheus.ExponentialBuckets(startBucketWithMicrosecond, 2, 15),
		}, []string{"component", "metric"}),
		logger: logger,
	}
}

func (c *ComponentProcessingDurationMetric) ExposeProcessingDuration(component string, state model.OperationState, duration time.Duration) {
	metricLabel := getMetricLabel(state)
	m, err := c.Collector.GetMetricWithLabelValues(component, metricLabel)
	if err != nil {
		c.logger.Errorf("ComponentProcessingDurationMetric: unable to retrieve metric with label=%s: %s", component, err.Error())
		return
	}
	m.Observe(float64(duration))
}

func getMetricLabel(state model.OperationState) string {
	switch state {
	case model.OperationStateDone:
		return "processing_duration_successful_microsecond"
	case model.OperationStateFailed:
		return "processing_duration_unsuccessful_microsecond"
	}
	return "undefined"
}
