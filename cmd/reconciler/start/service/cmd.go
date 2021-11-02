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
		Short: fmt.Sprintf("Start '%s' reconciler service", reconcilerName),
		Long:  fmt.Sprintf("CLI tool to start the Kyma '%s' component reconciler service", reconcilerName),
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
	ctx := cli.NewContext()
	workerPool, err := StartComponentReconciler(ctx, o, reconcilerName)
	if err != nil {
		return err
	}
	return StartWebserver(ctx, o, workerPool)
}
