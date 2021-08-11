package cmd

import (
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a Kyma component reconciler",
		Long:  "CLI tool to start a Kyma component reconciler",
	}

	return cmd
}
