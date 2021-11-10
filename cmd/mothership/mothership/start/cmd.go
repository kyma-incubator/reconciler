package cmd

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reconciler service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}

			//create enc-key before starting application registry (otherwise registry bootstrap will fail)
			if o.CreateEncyptionKey {
				encKeyFile, err := cli.NewEncryptionKey(true)
				if err != nil {
					o.Logger().Warnf("Failed to create encryption key file '%s'", encKeyFile)
					return err
				}
				o.Logger().Infof("New encryption key file created: %s", encKeyFile)
			}

			if err := o.InitApplicationRegistry(true); err != nil {
				return err
			}
			return Run(cli.NewContext(), o)
		},
	}
	cmd.Flags().IntVar(&o.Port, "server-port", 8080, "Webserver port")
	cmd.Flags().StringVar(&o.SSLCrt, "server-crt", "", "Path to SSL certificate file")
	cmd.Flags().StringVar(&o.SSLKey, "server-key", "", "Path to SSL key file")
	cmd.Flags().IntVarP(&o.MaxParallelOperations, "max-parallel", "", 0, "Maximal parallel reconciled components per cluster, 0 means unlimited")
	cmd.Flags().IntVarP(&o.Workers, "worker-count", "", 50, "Size of the reconciler worker pool")
	cmd.Flags().DurationVarP(&o.OrphanOperationTimeout, "orphan-timeout", "", 10*time.Minute, "Timeout until a processed operation which hasn't received status updates from its worker will be restarted")
	cmd.Flags().DurationVarP(&o.WatchInterval, "watch-interval", "", 1*time.Minute, "Size of the reconciler worker pool")
	cmd.Flags().DurationVarP(&o.ClusterReconcileInterval, "reconcile-interval", "", 5*time.Minute, "Defines the time when a cluster will to be reconciled since his last successful reconciliation")
	cmd.Flags().BoolVar(&o.CreateEncyptionKey, "create-encryption-key", false, "Create new encryption key file during startup")
	cmd.Flags().BoolVar(&o.Migrate, "migrate-database", false, "Migrate database to the latest release")
	cmd.Flags().BoolVar(&o.AuditLog, "audit-log", false, "Enable audit logging")
	cmd.Flags().StringVar(&o.AuditLogFile, "audit-log-file", "/var/log/mothership-audit-log.log", "Path for mothership audit log file")
	return cmd
}

func Run(ctx context.Context, o *Options) error {
	go func(ctx context.Context, o *Options) {
		err := startScheduler(ctx, o, viper.ConfigFileUsed())
		if err != nil {
			panic(err)
		}
	}(ctx, o)

	return startWebserver(ctx, o)
}
