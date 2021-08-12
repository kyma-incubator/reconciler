package cmd

import (
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test a Kyma component reconciler",
		Long:  "CLI tool to test a Kyma component reconciler",
	}

	return cmd
}
