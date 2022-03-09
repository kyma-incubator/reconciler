package metrics

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/prometheus/client_golang/prometheus"
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

func NewReconciliationStatusCollector() *ReconciliationStatusCollector {
	collector := &ReconciliationStatusCollector{
		reconciliationStatusGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "reconciliation_status",
			Help:      "Status of the reconciliation",
		}, []string{"runtime_id", "runtime_name", "global_accountid"}),
	}
	prometheus.MustRegister(collector)
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
		WithLabelValues(state.Cluster.RuntimeID, state.Cluster.Runtime.Name, fmt.Sprintf("%s", state.Cluster.Metadata.GlobalAccountID)).
		Set(status.ID)

	return nil
}
