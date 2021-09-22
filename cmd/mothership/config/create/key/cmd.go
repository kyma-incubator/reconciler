package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/spf13/cobra"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "key",
		Aliases: []string{"keys", "ke"},
		Short:   "Create a configuration key.",
		Long:    `Create a new entity or version of a configuration key.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o, args)
		},
	}

	cmd.Flags().StringVar(&o.DataType, "data-type", "string", fmt.Sprintf("Define data-type of the key (supported types are %s, %s, %s)",
		model.String, model.Integer, model.Boolean))
	cmd.Flags().BoolVar(&o.Encrypted, "encrypted", true, "Key values have to be encrypted")
	cmd.Flags().StringVar(&o.Validator, "validator", "", "Validator logic executed when setting a new value")
	cmd.Flags().StringVar(&o.Trigger, "trigger", "", "Trigger function executed when a value was added/changed")

	if err := cobra.MarkFlagRequired(cmd.Flags(), "data-type"); err != nil {
		panic(err) //would be an obvious bug and has to lead to a panic
	}

	return cmd
}

func Run(o *Options, keys []string) error {
	for _, key := range keys {
		newKey, err := createKey(o, key)
		if err != nil {
			return err
		}
		fmt.Printf("Key '%s' created\n", newKey.Key)
	}
	return nil
}

func createKey(o *Options, key string) (*model.KeyEntity, error) {
	dt, err := model.NewDataType(o.DataType)
	if err != nil {
		return nil, err
	}
	return o.Registry.KVRepository().CreateKey(&model.KeyEntity{
		Key:       key,
		DataType:  dt,
		Encrypted: o.Encrypted,
		Validator: o.Validator,
		Trigger:   o.Trigger,
		Username:  "!TODO!", //FIXME
	})
}
