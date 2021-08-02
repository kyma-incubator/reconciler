package cmd

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/spf13/cobra"
)

/*
* To create a component reconciler, please adjust the options.go file.
* Normally are no changes in this file required.
 */

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   o.Name,
		Short: fmt.Sprintf("Administrate Kyma '%s' reconcilers", o.Name),
		Long:  fmt.Sprintf("Administrative CLI tool for the Kyma '%s' reconciler service", o.Name),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o)
		},
	}

	return cmd
}

func Run(o *Options) error {
	//create component reconciler
	recon, err := service.NewComponentReconciler(o.Workspace, o.Verbose)
	if err != nil {
		return err
	}

	//configure component reconciler
	recon.WithDependencies(o.Dependencies...).
		//configure custom actions
		WithPreReconcileAction(o.PreAction).
		WithReconcileAction(o.ReconcileAction).
		WithPostReconcileAction(o.PostAction).
		//configure REST API server
		WithServerConfig(o.ServerConfig.Port, o.ServerConfig.SSLCrt, o.ServerConfig.SSLKey).
		//configure reconciliation
		WithWorkers(o.WorkerConfig.Workers, o.WorkerConfig.Timeout).
		WithRetry(o.RetryConfig.MaxRetries, o.RetryConfig.RetryDelay).
		//configure status updates send to mothership reconciler
		WithStatusUpdaterConfig(o.StatusUpdaterConfig.Interval, o.StatusUpdaterConfig.Timeout).
		//configure reconciliation progress-checks applied on target K8s cluster
		WithProgressTrackerConfig(o.ProgressTrackerConfig.Interval, o.ProgressTrackerConfig.Timeout)

	//start component reconciler service
	return recon.StartRemote(cli.NewContext())
}
