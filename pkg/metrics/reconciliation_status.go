package metrics

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	prometheusSubsystem = "reconciler"

	statusError            float64 = 0
	statusReady            float64 = 1
	statusReconcilePending float64 = 2
	statusReconciling      float64 = 3
	statusReconcileFailed  float64 = 4
)

// ReconciliationStatusCollector provides the following metrics:
// - reconciler_reconciliation_status{"cluster_id", "runtime_id", "global_account_id", "configuration_id"}
// These gauges show the status of the operation.
// The value of the gauge could be:
// 0 - Error
// 1 - Ready
// 2 - Reconcile Pending
// 3 - Reconciling
// 4 - Reconcile Failed
type ReconciliationStatusCollector struct {
	reconciliationStatusGauge *prometheus.GaugeVec
}

func NewReconciliationStatusCollector() *ReconciliationStatusCollector {
	return &ReconciliationStatusCollector{
		reconciliationStatusGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: prometheusSubsystem,
			Name:      "reconciliation_status",
			Help:      "Status of the reconciliation",
		}, []string{"cluster_id", "runtime_id", "cluster_version", "configuration_version"}),
	}
}

func (c *ReconciliationStatusCollector) Describe(ch chan<- *prometheus.Desc) {
	c.reconciliationStatusGauge.Describe(ch)
}

func (c *ReconciliationStatusCollector) Collect(ch chan<- prometheus.Metric) {
	c.reconciliationStatusGauge.Collect(ch)
}

func (c *ReconciliationStatusCollector) OnClusterStateUpdate(state *cluster.State) error {

	var resultValue float64
	switch state.Status.Status {
	case model.Error:
		resultValue = statusError
	case model.Ready:
		resultValue = statusReady
	case model.ReconcilePending:
		resultValue = statusReconcilePending
	case model.Reconciling:
		resultValue = statusReconciling
	case model.ReconcileFailed:
		resultValue = statusReconcileFailed
	}

	c.reconciliationStatusGauge.
		WithLabelValues(state.Cluster.Cluster, state.Cluster.RuntimeName, fmt.Sprintf("%d", state.Cluster.Version), fmt.Sprintf("%d", state.Configuration.Version)).
		Set(resultValue)

	return nil
}
