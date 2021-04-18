package cmd

import (
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get configuration entries",
	}
	return cmd
}
