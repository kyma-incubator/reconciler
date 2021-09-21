package cmd

import (
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
)

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create configuration entries",
	}
	return cmd
}
