package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"time"
)

type dbMetric struct {
	value float64
	name  string
}

type dbMetricInput *sql.DBStats
type metricDefinition func(i dbMetricInput) *dbMetric

type DbPoolCollector struct {
	conn              db.Connection
	logger            *zap.SugaredLogger
	collectionTimeout time.Duration
	desc              *prometheus.GaugeVec
	metricDefinitions []metricDefinition
}

const measurementAccuracyUnit = "milliseconds"
const defaultPoolName = "default"

const (
	dbInUse              = "in_use"
	dbIdle               = "idle"
	dbMaxIdleClosed      = "max_idle_closed"
	dbMaxIdleTimeClosed  = "max_idle_time_closed"
	dbMaxLifetimeClosed  = "max_lifetime_closed"
	dbMaxOpenConnections = "max_open_connections"
	dbOpenConnections    = "open_connections"
	dbWaitCount          = "wait_count"
	dbWaitDuration       = "wait_duration"
)

func NewDbPoolCollector(connPool db.Connection, logger *zap.SugaredLogger) *DbPoolCollector {
	return &DbPoolCollector{
		conn:              connPool,
		logger:            logger,
		collectionTimeout: 30 * time.Second,
		desc: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "db_pool_stats",
			Help:      "Stats from go SQL database pool",
		}, []string{"pool", "metric"}),
		metricDefinitions: []metricDefinition{
			func(i dbMetricInput) *dbMetric { return &dbMetric{float64(i.InUse), dbInUse} },
			func(i dbMetricInput) *dbMetric { return &dbMetric{float64(i.Idle), dbIdle} },
			func(i dbMetricInput) *dbMetric { return &dbMetric{float64(i.MaxIdleClosed), dbMaxIdleClosed} },
			func(i dbMetricInput) *dbMetric { return &dbMetric{float64(i.MaxIdleTimeClosed), dbMaxIdleTimeClosed} },
			func(i dbMetricInput) *dbMetric { return &dbMetric{float64(i.MaxLifetimeClosed), dbMaxLifetimeClosed} },
			func(i dbMetricInput) *dbMetric { return &dbMetric{float64(i.MaxOpenConnections), dbMaxOpenConnections} },
			func(i dbMetricInput) *dbMetric { return &dbMetric{float64(i.OpenConnections), dbOpenConnections} },
			func(i dbMetricInput) *dbMetric { return &dbMetric{float64(i.WaitCount), dbWaitCount} },
			func(i dbMetricInput) *dbMetric {
				return &dbMetric{
					float64(i.WaitDuration.Milliseconds()),
					fmt.Sprintf("%v_%s", dbWaitDuration, measurementAccuracyUnit),
				}
			},
		},
	}
}

func (c *DbPoolCollector) buildGaugeFromMetric(metric *dbMetric, ch chan *prometheus.Gauge) {
	m, err := c.desc.GetMetricWithLabelValues(defaultPoolName, metric.name)
	if err != nil {
		c.logger.Errorf("dbPoolCollector: unable to build gauge for %s(%s): %s", defaultPoolName, metric.name, err.Error())
		return
	}
	m.Set(metric.value)
	ch <- &m
}

func (c *DbPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	c.desc.Describe(ch)
}

// Collect implements the prometheus.Collector interface.
func (c *DbPoolCollector) Collect(ch chan<- prometheus.Metric) {

	if c.conn == nil {
		c.logger.Error("unable to register accessMetric: connection is nil")
		return
	}

	stats := c.conn.DBStats()
	if stats == nil {
		return
	}

	gauges := make(chan *prometheus.Gauge, len(c.metricDefinitions))
	defer close(gauges)

	ctx, cancel := context.WithTimeout(context.Background(), c.collectionTimeout)
	defer cancel()

	for _, metricDefinition := range c.metricDefinitions {
		go c.buildGaugeFromMetric(metricDefinition(stats), gauges)
	}

	for range c.metricDefinitions {
		select {
		case gauge := <-gauges:
			c.logger.Debugf("finished calculating db gauge: %s", (*gauge).Desc().String())
		case <-ctx.Done():
			c.logger.Error(ctx.Err())
		}
	}

	c.desc.Collect(ch)
}
