package cmd

import (
	"time"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
)

const (
	paramContractVersion = "contractVersion"
	paramCluster         = "cluster"
	paramConfigVersion   = "configVersion"
	paramOffset          = "offset"
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
			return Run(o)
		},
	}
	cmd.Flags().IntVar(&o.Port, "port", 8080, "Webserver port")
	cmd.Flags().StringVar(&o.SSLCrt, "crt", "", "Path to SSL certificate file")
	cmd.Flags().StringVar(&o.SSLKey, "key", "", "Path to SSL key file")
	cmd.Flags().IntVarP(&o.Workers, "workers", "", 50, "Size of the reconciler worker pool")
	cmd.Flags().DurationVarP(&o.WatchInterval, "watch-interval", "", 1*time.Minute, "Size of the reconciler worker pool")
	cmd.Flags().DurationVarP(&o.ClusterReconcileInterval, "reconcile-interval", "", 5*time.Minute, "Defines the time when a cluster will to be reconciled since his last successful reconciliation")
	cmd.Flags().StringVar(&o.ReconcilersCfgPath, "reconcilers", "", "Path to component reconcilers configuration file")
	return cmd
}

func Run(o *Options) error {
	ctx := cli.NewContext()

	if err := startWebserver(ctx, o); err != nil {
		return err
	}

	return startScheduler(ctx, o)
}
