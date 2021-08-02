package start

import (
	"github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/spf13/cobra"
	"time"
)

const defaultTimeout = 10 * time.Minute //max time a reconciliation process is allowed to take

func NewCmd(o *reconciler.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a Kyma component reconciler",
		Long:  "CLI tool to start a Kyma component reconciler",
	}

	//worker pool configuration
	cmd.PersistentFlags().IntVar(&o.WorkerConfig.Workers, "worker-count", 50,
		"Number of in parallel running reconciliation workers")
	cmd.PersistentFlags().DurationVar(&o.WorkerConfig.Timeout, "worker-timeout", defaultTimeout,
		"Maximal time a worker will run before a reconciliation will be stopped")

	//REST API configuration
	cmd.PersistentFlags().IntVar(&o.ServerConfig.Port, "server-port", 8080,
		"Port of the REST API")
	cmd.PersistentFlags().StringVar(&o.ServerConfig.SSLCrt, "server-crt", "",
		"Path to SSL certificate file used for secure REST API communication")
	cmd.PersistentFlags().StringVar(&o.ServerConfig.SSLKey, "server-key", "",
		"Path to SSL key file used for secure REST API communication")

	//retry configuration
	cmd.PersistentFlags().IntVar(&o.RetryConfig.MaxRetries, "retries-max", 5,
		"Number of retries until the reconciler will report a reconciliation as consistently failing")
	cmd.PersistentFlags().DurationVar(&o.RetryConfig.RetryDelay, "retries-delay", 30*time.Second,
		"Delay between each reconciliation retry")

	//status-updater configuration
	cmd.PersistentFlags().DurationVar(&o.StatusUpdaterConfig.Interval, "status-interval", 30*time.Second,
		"Interval to report the latest reconciliation process status to the mothership reconciler")
	o.StatusUpdaterConfig.Timeout = o.WorkerConfig.Timeout //coupled to reconcile-timeout

	//progress-tracker configuration
	cmd.PersistentFlags().DurationVar(&o.ProgressTrackerConfig.Interval, "progress-interval", 15*time.Second,
		"Interval to verify the installation progress of a deployed Kubernetes resource")
	o.ProgressTrackerConfig.Timeout = o.WorkerConfig.Timeout //coupled to reconcile-timeout

	//file cache for Kyma sources
	cmd.PersistentFlags().StringVar(&o.Workspace, "workspace", ".",
		"Workspace directory used to cache Kyma sources")

	return cmd
}
