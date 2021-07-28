package cmd

import (
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/spf13/cobra"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the component reconciler service",
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
	cmd.Flags().StringVar(&o.Workspace, "workspace", ".", "Workspace directory used to cache Kyma sources")
	return cmd
}

func Run(o *Options) error {
	ctx := cli.NewContext()

	chartProvider := o.Registry.ChartProvider()
	if err := chartProvider.ChangeWorkspace(o.Workspace); err != nil {
		return err
	}

	return service.NewComponentReconciler(chartProvider).
		WithServerConfig(o.Port, o.SSLCrt, o.SSLKey).
		StartRemote(ctx)
}
