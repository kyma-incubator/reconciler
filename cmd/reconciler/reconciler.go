package start

import (
	startCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/start"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/spf13/cobra"
)

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconciler",
		Short: "Administrate Kyma component reconcilers",
		Long:  "Administrative CLI tool for the Kyma component reconcilers",
	}

	cmd.AddCommand(startCmd.NewCmd(reconciler.NewOptions(o)))

	return cmd
}
