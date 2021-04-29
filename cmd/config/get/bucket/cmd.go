package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
)

//NewCmd creates a new apply command
func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bucket",
		Short: "Get configuration entry buckets.",
		Long:  `Get available buckets and their containing configuration entries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(o)
		},
	}
	return cmd
}

func Run(o *cli.Options) error {
	buckets, err := o.Repository().Buckets()
	if err != nil {
		return err
	}
	for _, bucket := range buckets {
		fmt.Println(bucket)
	}
	return nil
}
