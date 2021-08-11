package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/internal/cli"
	reconCli "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/spf13/cobra"
)

func NewCmd(o *reconCli.Options, reconcilerName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   reconcilerName,
		Short: fmt.Sprintf("Test '%s' reconciler service", reconcilerName),
		Long:  fmt.Sprintf("CLI tool to test the Kyma '%s' component reconciler service", reconcilerName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o, reconcilerName)
		},
	}

	return cmd
}

func Run(o *reconCli.Options, reconcilerName string) error {
	recon, err := reconCli.NewComponentReconciler(o, reconcilerName)
	if err != nil {
		return err
	}

	o.Logger().Infof("Starting component reconciler '%s' in debug mode", reconcilerName)
	if err := recon.Debug(); err != nil {
		return err
	}
	return recon.StartRemote(cli.NewContext())
}
