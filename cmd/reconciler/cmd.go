package cmd

import (
	startCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/start"
	startSvcCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/start/service"
	testCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/test"
	testSvcCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/test/service"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	reconcilerRegistry "github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/spf13/cobra"
	"time"

	//imports loader.go which ensures that all available component reconcilers are added to the reconciler registry:
	_ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"
)

const defaultTimeout = 10 * time.Minute //max time a rec onciliation process is allowed to take

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconciler",
		Short: "Administrate Kyma component reconcilers",
		Long:  "Administrative CLI tool for the Kyma component reconcilers",
	}

	reconcilerOpts := reconciler.NewOptions(o) //decorate options with reconciler-specific options

	//worker pool configuration
	cmd.PersistentFlags().IntVar(&reconcilerOpts.WorkerConfig.Workers, "worker-count", 50,
		"Number of in parallel running reconciliation workers")
	cmd.PersistentFlags().DurationVar(&reconcilerOpts.WorkerConfig.Timeout, "worker-timeout", defaultTimeout,
		"Maximal time a worker will run before a reconciliation will be stopped")

	//REST API configuration
	cmd.PersistentFlags().IntVar(&reconcilerOpts.ServerConfig.Port, "server-port", 8080,
		"Port of the REST API")
	cmd.PersistentFlags().StringVar(&reconcilerOpts.ServerConfig.SSLCrt, "server-crt", "",
		"Path to SSL certificate file used for secure REST API communication")
	cmd.PersistentFlags().StringVar(&reconcilerOpts.ServerConfig.SSLKey, "server-key", "",
		"Path to SSL key file used for secure REST API communication")

	//retry configuration
	cmd.PersistentFlags().IntVar(&reconcilerOpts.RetryConfig.MaxRetries, "retries-max", 5,
		"Number of retries until the reconciler will report a reconciliation as consistently failing")
	cmd.PersistentFlags().DurationVar(&reconcilerOpts.RetryConfig.RetryDelay, "retries-delay", 30*time.Second,
		"Delay between each reconciliation retry")

	//status-updater configuration
	cmd.PersistentFlags().DurationVar(&reconcilerOpts.StatusUpdaterConfig.Interval, "status-interval", 30*time.Second,
		"Interval to report the latest reconciliation process status to the mothership reconciler")
	reconcilerOpts.StatusUpdaterConfig.Timeout = reconcilerOpts.WorkerConfig.Timeout //coupled to reconcile-timeout

	//progress-tracker configuration
	cmd.PersistentFlags().DurationVar(&reconcilerOpts.ProgressTrackerConfig.Interval, "progress-interval", 15*time.Second,
		"Interval to verify the installation progress of a deployed Kubernetes resource")
	reconcilerOpts.ProgressTrackerConfig.Timeout = reconcilerOpts.WorkerConfig.Timeout //coupled to reconcile-timeout

	//file cache for Kyma sources
	cmd.PersistentFlags().StringVar(&reconcilerOpts.Workspace, "workspace", ".",
		"Workspace directory used to cache Kyma sources")

	startCommand := startCmd.NewCmd()
	cmd.AddCommand(startCommand)
	//register component reconcilers in start command:
	for _, reconcilerName := range reconcilerRegistry.RegisteredReconcilers() {
		startCommand.AddCommand(startSvcCmd.NewCmd(reconcilerOpts, reconcilerName))
	}

	testCommand := testCmd.NewCmd()
	cmd.AddCommand(testCommand)
	//register component reconcilers in start command:
	for _, reconcilerName := range reconcilerRegistry.RegisteredReconcilers() {
		testCommand.AddCommand(testSvcCmd.NewCmd(testSvcCmd.NewOptions(reconcilerOpts), reconcilerName))
	}

	return cmd
}
