package metrics

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	prometheusSubsystem = "reconciler"
)

// ReconciliationStatusCollector provides the following metrics:
// - reconciler_reconciliation_status{"runtime_id", "runtime_name", "cluster_version", "configuration_version"}
// These gauges show the status of the reconciliation.
// The value of the gauge could be:
// 0 - Reconcile Error
// 1 - Ready
// 2 - Reconcile Pending
// 3 - Reconciling
// 4 - Reconcile Disabled
// 5 - Delete Pending
// 6 - Deleting
// 7 - Delete Error
// 8 - Deleted
type ReconciliationStatusCollector struct {
	reconciliationStatusGauge *prometheus.GaugeVec
}

func NewReconciliationStatusCollector(logger *zap.SugaredLogger) *ReconciliationStatusCollector {
	collector := &ReconciliationStatusCollector{
		reconciliationStatusGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "reconciliation_status",
			Help:      "Status of the reconciliation",
		}, []string{"runtime_id", "runtime_name"}),
	}
	err := prometheus.Register(collector)
	switch err := err.(type) {
	case prometheus.AlreadyRegisteredError:
		logger.Warnf("skipping registration of waiting/ready metrics as they were already registered, existing: %v",
			err.ExistingCollector)
		return collector
	}
	if err != nil {
		panic(err)
	}
	return collector
}

func (c *ReconciliationStatusCollector) Describe(ch chan<- *prometheus.Desc) {
	c.reconciliationStatusGauge.Describe(ch)
}

func (c *ReconciliationStatusCollector) Collect(ch chan<- prometheus.Metric) {
	c.reconciliationStatusGauge.Collect(ch)
}

func (c *ReconciliationStatusCollector) OnClusterStateUpdate(state *cluster.State) error {
	status, err := state.Status.GetClusterStatus()
	if err != nil {
		return err
	}

	c.reconciliationStatusGauge.
		WithLabelValues(state.Cluster.RuntimeID, state.Cluster.Runtime.Name).
		Set(status.ID)

	return nil
}
