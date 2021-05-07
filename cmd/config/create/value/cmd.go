package cmd

import (
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "value",
		Aliases: []string{"values", "va"},
		Short:   "Create a configuration value.",
		Long:    `Create a new entity or version of a configuration value.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("One value has to provided for key '%s' (version %d): '%s'",
					o.Key, o.KeyVersion, strings.Join(args, "', '"))
			}
			return Run(o, args[0])
		},
	}

	cmd.Flags().StringVarP(&o.Bucket, "bucket", "", "global", "Bucket this value will be added to")
	cmd.Flags().StringVar(&o.Key, "key", "", "Key of the value")
	cmd.Flags().Int64Var(&o.KeyVersion, "key-version", 0, "Key version")

	if err := cobra.MarkFlagRequired(cmd.Flags(), "bucket"); err != nil {
		panic(err) //would be an obvious bug and has to lead to a panic
	}
	if err := cobra.MarkFlagRequired(cmd.Flags(), "key"); err != nil {
		panic(err) //would be an obvious bug and has to lead to a panic
	}
	if err := cobra.MarkFlagRequired(cmd.Flags(), "key-version"); err != nil {
		panic(err) //would be an obvious bug and has to lead to a panic
	}

	return cmd
}

func Run(o *Options, val string) error {
	key, err := o.Repository().Key(o.Key, o.KeyVersion)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to retrieve key '%s' (version %d)", o.Key, o.KeyVersion))
	}
	value, err := o.Repository().CreateValue(&config.ValueEntity{
		Bucket:     o.Bucket,
		Key:        key.Key,
		KeyVersion: key.Version,
		Value:      val,
		Username:   "!TODO!", //FIXME
	})
	if err != nil {
		return err
	}

	fmt.Printf("Value '%s' created (bucket: %s / key: %s - version %d)\n", value.Value, value.Bucket, value.Key, value.KeyVersion)
	return nil
}
