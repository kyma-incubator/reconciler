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
	const startBucketWithMillisecond = 1e3
	return &ComponentProcessingDurationMetric{
		Collector: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: prometheusSubsystem,
			Name:      "processing_time",
			Help:      "Processing time of operations",
			Buckets:   prometheus.ExponentialBuckets(startBucketWithMillisecond, 2, 10),
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
	durationToMillisecond := duration / 1e6
	m.Observe(float64(durationToMillisecond))
}

func getMetricLabel(state model.OperationState) string {
	switch state {
	case model.OperationStateDone:
		return "processing_duration_successful_millisecond"
	case model.OperationStateFailed:
		return "processing_duration_unsuccessful_millisecond"
	}
	return "undefined"
}
