package cmd

import (
	"github.com/kyma-incubator/reconciler/pkg/compreconciler"
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
	return cmd
}

func Run(o *Options) error {
	return compreconciler.NewComponentReconciler().Start()
}
