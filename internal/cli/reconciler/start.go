package reconciler

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
)

func NewComponentReconciler(o *Options, reconcilerName string) (*service.ComponentReconciler, error) {
	recon, err := service.GetReconciler(reconcilerName)
	if err != nil {
		return nil, err
	}

	if o.Verbose {
		if err := recon.Debug(); err != nil {
			return nil, errors.Wrap(err, "Failed to enable debug mode")
		}
	}

	recon.WithWorkspace(o.Workspace).
		//configure REST API server
		WithServerConfig(o.ServerConfig.Port, o.ServerConfig.SSLCrtFile, o.ServerConfig.SSLKeyFile).
		//configure reconciliation worker pool + retry-behaviour
		WithWorkers(o.WorkerConfig.Workers, o.WorkerConfig.Timeout).
		WithRetry(o.RetryConfig.MaxRetries, o.RetryConfig.RetryDelay).
		//configure status updates send to mothership reconciler
		WithHeartbeatSenderConfig(o.HeartbeatSenderConfig.Interval, o.HeartbeatSenderConfig.Timeout).
		//configure reconciliation progress-checks applied on target K8s cluster
		WithProgressTrackerConfig(o.ProgressTrackerConfig.Interval, o.ProgressTrackerConfig.Timeout)

	return recon, nil
}
