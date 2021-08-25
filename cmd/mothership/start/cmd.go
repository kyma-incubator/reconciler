package cmd

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	paramContractVersion = "contractVersion"
	paramCluster         = "cluster"
	paramConfigVersion   = "configVersion"
	paramOffset          = "offset"
	paramSchedulingID    = "schedulingID"
	paramCorrelationID   = "correlationID"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reconciler service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.InitApplicationRegistry(true); err != nil {
				return err
			}
			return Run(o)
		},
	}
	cmd.Flags().IntVar(&o.Port, "server-port", 8080, "Webserver port")
	cmd.Flags().StringVar(&o.SSLCrt, "server-crt", "", "Path to SSL certificate file")
	cmd.Flags().StringVar(&o.SSLKey, "server-key", "", "Path to SSL key file")
	cmd.Flags().IntVarP(&o.Workers, "worker-count", "", 50, "Size of the reconciler worker pool")
	cmd.Flags().DurationVarP(&o.WatchInterval, "watch-interval", "", 1*time.Minute, "Size of the reconciler worker pool")
	cmd.Flags().DurationVarP(&o.ClusterReconcileInterval, "reconcile-interval", "", 5*time.Minute, "Defines the time when a cluster will to be reconciled since his last successful reconciliation")
	cmd.Flags().StringVar(&o.ReconcilersCfgPath, "reconcilers", "", "Path to component reconcilers configuration file")
	cmd.Flags().BoolVar(&o.CreateEncyptionKey, "create-encryption-key", false, "Create a new encryption file during startup")
	return cmd
}

func Run(o *Options) error {
	if o.CreateEncyptionKey {
		err := cli.NewEncryptionKey(true)
		if err == nil {
			o.Logger().Infof("New encryption key file created")
		} else {
			o.Logger().Warnf("Failed to create encryption key file")
			return err
		}
	}

	ctx := cli.NewContext()

	go func(ctx context.Context, o *Options) {
		err := startScheduler(ctx, o, viper.ConfigFileUsed())
		if err != nil {
			panic(err)
		}
	}(ctx, o)

	return startWebserver(ctx, o)
}
