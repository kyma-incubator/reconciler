package cmd

import (
	"os"
	"time"

	getCmd "github.com/kyma-incubator/reconciler/cmd/config/get"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/config"
	"github.com/spf13/cobra"
)

//NewCmd creates a new apply command
func NewCmd(o *getCmd.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bucket",
		Aliases: []string{"buckets", "bu"},
		Short:   "Get configuration buckets.",
		Long:    `List configuration buckets or get a bucket inclusive its containing configuration entries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(o, args)
		},
	}
	return cmd
}

func Run(o *getCmd.Options, bucketFilter []string) error {
	if err := o.Validate(); err != nil {
		return err
	}

	buckets, err := o.Repository().Buckets()
	if err != nil {
		return err
	}

	return render(o, buckets)
}

func render(o *getCmd.Options, buckets []*config.BucketEntity) error {

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
	return formatter.Output(os.Stdout)
}
