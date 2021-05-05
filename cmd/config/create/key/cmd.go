package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/config"
	"github.com/spf13/cobra"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "key",
		Aliases: []string{"keys", "ke"},
		Short:   "Create a configuration key.",
		Long:    `Create a new entity or version of a configuration key.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(o, args)
		},
	}

	cmd.Flags().StringVar(&o.DataType, "data-type", "", fmt.Sprintf("Define data-type of the key (supported types are %s, %s, %s)",
		config.String, config.Integer, config.Boolean))

	if err := cobra.MarkFlagRequired(cmd.Flags(), "data-type"); err != nil {
		panic(err) //would be an obvious bug and has to lead to a panic
	}

	return cmd
}

func Run(o *Options, keys []string) error {
	if err := o.Validate(); err != nil {
		return err
	}

	for _, key := range keys {
		_, err := o.Repository().LatestKey(key)
		if err == nil {
			//update
		}
		if config.IsNotFoundError(err) {
			//create
		}
		return err
	}

	return nil
}
