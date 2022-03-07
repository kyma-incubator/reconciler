package cmd

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
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

			if o.StopAfterMigration {
				return db.MigrateDatabase(viper.ConfigFileUsed(), o.Verbose)
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
	cmd.Flags().DurationVar(&o.PurgeEntitiesOlderThan, "purge-older-than", 14*24*time.Hour, "[Deprecated] Defines the minimum age of entities like Reconciliations and Operations that will be removed")
	cmd.Flags().IntVar(&o.KeepLatestEntitiesCount, "cleaner-keep-n-latest", 0, "Defines the count of most recent entities the cleaner won't remove during it's operation")                            //It's set to zero to disable it by default. Change to a proper value once this mechanism is enabled in the environments.
	cmd.Flags().IntVar(&o.KeepUnsuccessfulEntitiesDays, "cleaner-keep-failed-ops-days", 0, "Defines the number of days for which the cleaner keeps entities with unsuccessful status before removal") //It's set to zero to disable it by default. Change to a proper value once this mechanism is enabled in the environments.
	cmd.Flags().DurationVar(&o.CleanerInterval, "cleaner-interval", 14*time.Hour, "Define the time interval when the cleaner will be looking for entities to remove")
	cmd.Flags().BoolVar(&o.CreateEncyptionKey, "create-encryption-key", false, "Create new encryption key file during startup")
	cmd.Flags().BoolVar(&o.Migrate, "migrate-database", false, "Migrate database to the latest release")
	cmd.Flags().BoolVar(&o.AuditLog, "audit-log", false, "Enable audit logging")
	cmd.Flags().StringVar(&o.AuditLogFile, "audit-log-file", "/var/log/auditlog/mothership-audit.log", "Path for mothership audit log file")
	cmd.Flags().StringVar(&o.AuditLogTenantID, "audit-log-tenant-id", "", "tenant id for audit logging")
	cmd.Flags().BoolVar(&o.StopAfterMigration, "stop-after-migrate", false, "Stop mothership after database migration to the latest release")
	return cmd
}

func Run(ctx context.Context, o *Options) error {
	schedulerCfg, err := parseSchedulerConfig(viper.ConfigFileUsed())
	if err != nil {
		return err
	}
	o.ReconcilerList = getReconcilers(schedulerCfg)
	go func(ctx context.Context, o *Options) {
		err = startScheduler(ctx, o, schedulerCfg)
		if err != nil {
			panic(err)
		}
	}(ctx, o)

	metrics.RegisterAll(o.Registry.Inventory(), o.Registry.OccupancyRepository(), o.ReconcilerList, o.Logger())

	processingDurationCollector := metrics.NewProcessingDurationCollector(o.Registry.ReconciliationRepository(), o.Logger())
	err = prometheus.Register(processingDurationCollector.ProcessingDurationHistogram)
	if err != nil {
		o.Logger().Errorf("Prometheus can't register ProcessingDurationHistogram")
	}

	webserver := NewWebserver(processingDurationCollector)
	return webserver.startWebserver(ctx, o)
}
