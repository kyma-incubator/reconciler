package cmd

import (
	"os"
	"time"

	getCmd "github.com/kyma-incubator/reconciler/cmd/config/get"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
)

//NewCmd creates a new apply command
func NewCmd(o *getCmd.Options) *cobra.Command {
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

func Run(o *getCmd.Options) error {
	if err := o.Validate(); err != nil {
		return err
	}

	buckets, err := o.Repository().Buckets()
	if err != nil {
		return err
	}

	//print formatted output
	formatter, err := cli.NewOutputFormatter(o.OutputFormat)
	if err != nil {
		return err
	}

	if err := formatter.Header("Bucket", "Created by", "Created at"); err != nil {
		return err
	}
	for _, bucket := range buckets {
		if err := formatter.AddRow(bucket.Bucket, bucket.Username, bucket.Created.Format(time.RFC822Z)); err != nil {
			return err
		}
	}
	formatter.Output(os.Stdout)

	return nil
}
