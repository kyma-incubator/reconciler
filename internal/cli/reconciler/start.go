package reconciler

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func NewComponentReconciler(o *Options, reconcilerName string) (*service.ComponentReconciler, error) {
	recon, err := service.GetReconciler(reconcilerName)
	if err != nil {
		return nil, err
	}

	if o.Verbose {
		recon.Debug()
	}

	recon.WithWorkspace(o.Workspace).
		//configure reconciliation worker pool + retry-behaviour
		WithWorkers(o.WorkerConfig.Workers, o.WorkerConfig.Timeout).
		WithRetryDelay(o.RetryConfig.RetryDelay).
		//configure status updates send to mothership reconciler
		WithHeartbeatSenderConfig(o.HeartbeatSenderConfig.Interval, o.HeartbeatSenderConfig.Timeout).
		//configure reconciliation progress-checks applied on target K8s cluster
		WithProgressTrackerConfig(o.ProgressTrackerConfig.Interval, o.ProgressTrackerConfig.Timeout)

	return recon, nil
}
