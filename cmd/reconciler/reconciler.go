package cmd

import (
	startCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/start"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
)

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconciler",
		Short: "Kyma component reconciler",
		Long:  "Administrative CLI tool for the Kyma component reconcilers",
	}

	cmd.AddCommand(startCmd.NewCmd(startCmd.NewOptions(o)))

	return cmd
}
