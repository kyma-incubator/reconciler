package cmd

import (
	reconCli "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/spf13/cobra"
)

func NewCmd(o *reconCli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a Kyma component reconciler",
		Long:  "CLI tool to start a Kyma component reconciler",
	}

	cmd.PersistentFlags().BoolVar(&o.DryRun, "dry-run", false, "Dry run / render manifests only")

	return cmd
}
