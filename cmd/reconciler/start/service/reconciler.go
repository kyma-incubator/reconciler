package cmd

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"

	reconCli "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func StartComponentReconciler(ctx context.Context, o *reconCli.Options, reconcilerName string) (*service.WorkerPool, *service.OccupancyTracker, error) {
	if o.DryRun {
		service.EnableReconcilerDryRun()
	}
	if o.Verbose {
		service.EnableDebug()
	}
	durationMetric := metrics.NewComponentProcessingDurationMetric(o.Logger())
	err := prometheus.Register(durationMetric.Collector)
	if err != nil {
		return nil, nil, err
	}
	reconcilerMetricsSet := metrics.NewReconcilerMetricsSet(durationMetric)
	recon, err := reconCli.NewComponentReconciler(o, reconcilerName, reconcilerMetricsSet)
	if err != nil {
		return nil, nil, err
	}

	o.Logger().Infof("Starting component reconciler '%s'", reconcilerName)
	return recon.StartRemote(ctx, reconcilerName)
}
