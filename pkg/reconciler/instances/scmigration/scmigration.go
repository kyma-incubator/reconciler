package scmigration

import (
	startservice "github.com/kyma-incubator/reconciler/cmd/reconciler/start/service"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/prometheus/client_golang/prometheus"
)

const ReconcilerName = "sc-migration"

var (
	migratedInstancesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "migrated_instances_total",
			Help: "Migrated instances total",
		},
		[]string{"instance_id"},
	)

	migratedInstancesFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "migrated_instances_failures",
			Help: "Migrated instances failures",
		},
		[]string{"instance_id"},
	)

	svcatInstancesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "svcat_instances_total",
			Help: "Service catalog instances processed total",
		},
		[]string{"instance_id"},
	)

	smInstancesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sm_instances_total",
			Help: "Service manager instances processed total",
		},
		[]string{"instance_id"},
	)

	migratedBindingsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "migrated_bindings_total",
			Help: "Migrated bindings total",
		},
		[]string{"instance_id"},
	)

	migratedBindingsFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "migrated_bindings_failures",
			Help: "Migrated bindings failures",
		},
		[]string{"instance_id"},
	)

	svcatBindingsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "svcat_bindings_total",
			Help: "Service catalog bindings processed total",
		},
		[]string{"instance_id"},
	)

	smBindingsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sm_bindings_total",
			Help: "Service manager bindings processed total",
		},
		[]string{"instance_id"},
	)
)

//nolint:gochecknoinits
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}
	startservice.EnableMetrics = true
	prometheus.MustRegister(migratedInstancesTotal)
	prometheus.MustRegister(svcatInstancesTotal)
	prometheus.MustRegister(smInstancesTotal)
	prometheus.MustRegister(migratedBindingsTotal)
	prometheus.MustRegister(svcatBindingsTotal)
	prometheus.MustRegister(smBindingsTotal)
	reconciler.WithReconcileAction(&reconcileAction{})
}
