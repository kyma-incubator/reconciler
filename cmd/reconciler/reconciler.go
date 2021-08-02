package cmd

import (
	example "github.com/kyma-incubator/reconciler/cmd/reconciler/example"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/spf13/cobra"
	"time"
)

const defaultTimeout = 10 * time.Minute //max time a reconciliation process is allowed to take

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconciler",
		Short: "Administrate Kyma component reconcilers",
		Long:  "Administrative CLI tool for the Kyma component reconcilers",
	}

	//decorate CLI options with component reconciler attributes
	reconOpts := reconciler.NewOptions(o)

	//worker pool configuration
	cmd.PersistentFlags().IntVar(&reconOpts.WorkerConfig.Workers, "worker-count", 50,
		"Number of in parallel running reconciliation workers")
	cmd.PersistentFlags().DurationVar(&reconOpts.WorkerConfig.Timeout, "worker-timeout", defaultTimeout,
		"Maximal time a worker will run before a reconciliation will be stopped")

	//REST API configuration
	cmd.PersistentFlags().IntVar(&reconOpts.ServerConfig.Port, "server-port", 8080,
		"Port of the REST API")
	cmd.PersistentFlags().StringVar(&reconOpts.ServerConfig.SSLCrt, "server-crt", "",
		"Path to SSL certificate file used for secure REST API communication")
	cmd.PersistentFlags().StringVar(&reconOpts.ServerConfig.SSLKey, "server-key", "",
		"Path to SSL key file used for secure REST API communication")

	//retry configuration
	cmd.PersistentFlags().IntVar(&reconOpts.RetryConfig.MaxRetries, "retries-max", 5,
		"Number of retries until the reconciler will report a reconciliation as consistently failing")
	cmd.PersistentFlags().DurationVar(&reconOpts.RetryConfig.RetryDelay, "retries-delay", 30*time.Second,
		"Delay between each reconciliation retry")

	//status-updater configuration
	cmd.PersistentFlags().DurationVar(&reconOpts.StatusUpdaterConfig.Interval, "status-interval", 30*time.Second,
		"Interval to report the latest reconciliation process status to the mothership reconciler")
	reconOpts.StatusUpdaterConfig.Timeout = reconOpts.WorkerConfig.Timeout //coupled to reconcile-timeout

	//progress-tracker configuration
	cmd.PersistentFlags().DurationVar(&reconOpts.ProgressTrackerConfig.Interval, "progress-interval", 15*time.Second,
		"Interval to verify the installation progress of a deployed Kubernetes resource")
	reconOpts.ProgressTrackerConfig.Timeout = reconOpts.WorkerConfig.Timeout //coupled to reconcile-timeout

	//file cache for Kyma sources
	cmd.PersistentFlags().StringVar(&reconOpts.Workspace, "workspace", ".",
		"Workspace directory used to cache Kyma sources")

	//register component reconcilers
	registerComponentReconcilers(cmd, reconOpts)

	return cmd
}

func registerComponentReconcilers(cmd *cobra.Command, o *reconciler.Options) {
	cmd.AddCommand(example.NewCmd(example.NewOptions(o)))
}
