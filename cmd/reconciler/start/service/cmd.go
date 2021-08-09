package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/internal/cli"
	reconCli "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewCmd(o *reconCli.Options, reconcilerName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   reconcilerName,
		Short: fmt.Sprintf("Start '%s' reconciler service", reconcilerName),
		Long:  fmt.Sprintf("CLI tool to start the Kyma '%s' component reconciler service", reconcilerName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o, reconcilerName)
		},
	}

	return cmd
}

func Run(o *reconCli.Options, reconcilerName string) error {
	recon, err := service.GetReconciler(reconcilerName)
	if err != nil {
		return err
	}

	if o.Verbose {
		if err := recon.Debug(); err != nil {
			return errors.Wrap(err, "Failed to enable debug mode")
		}
	}

	recon.WithWorkspace(o.Workspace).
		//configure REST API server
		WithServerConfig(o.ServerConfig.Port, o.ServerConfig.SSLCrt, o.ServerConfig.SSLKey).
		//configure reconciliation worker pool + retry-behaviour
		WithWorkers(o.WorkerConfig.Workers, o.WorkerConfig.Timeout).
		WithRetry(o.RetryConfig.MaxRetries, o.RetryConfig.RetryDelay).
		//configure status updates send to mothership reconciler
		WithStatusUpdaterConfig(o.StatusUpdaterConfig.Interval, o.StatusUpdaterConfig.Timeout).
		//configure reconciliation progress-checks applied on target K8s cluster
		WithProgressTrackerConfig(o.ProgressTrackerConfig.Interval, o.ProgressTrackerConfig.Timeout)

	//start component reconciler service
	return recon.StartRemote(cli.NewContext())
}
