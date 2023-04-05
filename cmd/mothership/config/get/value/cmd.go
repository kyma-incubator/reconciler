package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/spf13/cobra"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "value",
		Aliases: []string{"values", "va"},
		Short:   "Get configuration value.",
		Long:    `List configuration values or get a value.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o, args)
		},
	}
	cmd.Flags().BoolVar(&o.History, "history", false, "Show history of configuration value")
	cmd.Flags().StringVar(&o.Key, "key", "", "Key name")
	cmd.Flags().Int64Var(&o.KeyVersion, "key-version", 0, "Key version")

	return cmd
}

func Run(o *Options, _ []string) error {
	key, err := getKey(o)
	if err != nil {
		return err
	}

	valueProcessor, err := newValueProcessor(o.Registry.KVRepository(), key)
	if err != nil {
		return err
	}

	if o.History {
		valueProcessor.withHistory()
	}

	values, err := valueProcessor.get()
	if err != nil {
		return err
	}

	// render all keys (without values)
	return renderValues(o, values)
}

func renderValues(o *Options, values []*model.ValueEntity) error {
	formatter, err := cli.NewOutputFormatter(o.OutputFormat)
	if err != nil {
		return err
	}

	if err := formatter.Header("Bucket", "Value", "Data Type", "Created by",
		"Created at (UTC)", "Version"); err != nil {
		return err
	}
	for _, value := range values {
		if err := formatter.AddRow(value.Bucket, value.Value, value.DataType, value.Username,
			value.Created.Format(time.RFC822Z), value.Version); err != nil {
			return err
		}
	}
	return formatter.Output(os.Stdout)
}

func getKey(o *Options) (*model.KeyEntity, error) {
	if o.Key != "" && o.KeyVersion > 0 {
		return o.Registry.KVRepository().Key(o.Key, o.KeyVersion)
	}
	if o.KeyVersion > 0 {
		return o.Registry.KVRepository().KeyByVersion(o.KeyVersion)
	}
	if o.Key != "" {
		return o.Registry.KVRepository().LatestKey(o.Key)
	}
	return nil, fmt.Errorf("Cannot resolve key: please provide either key, key-version or both")
}
