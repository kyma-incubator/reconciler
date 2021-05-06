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

	cmd.Flags().StringVar(&o.DataType, "data-type", "string", fmt.Sprintf("Define data-type of the key (supported types are %s, %s, %s)",
		config.String, config.Integer, config.Boolean))
	cmd.Flags().BoolVar(&o.Encrypted, "encrypted", true, "Key values have to be encrypted")
	cmd.Flags().StringVar(&o.Validator, "validator", "", "Validator logic executed when setting a new value")
	cmd.Flags().StringVar(&o.Trigger, "trigger", "", "Trigger function executed when a value was added/changed")

	return cmd
}

func Run(o *Options, keys []string) error {
	var newKey *config.KeyEntity
	for _, key := range keys {
		existingKey, err := o.Repository().LatestKey(key)
		if err == nil { //key found: update it
			newKey, err = updateKey(o, existingKey)
			if err != nil {
				return err
			}
			fmt.Printf("Key '%s' updated (version: %d)\n", newKey.Key, newKey.Version)
		} else if config.IsNotFoundError(err) { //key doesn't exist: create it
			newKey, err = createKey(o, key)
			if err != nil {
				return err
			}
			fmt.Printf("Key '%s' created\n", newKey.Key)
		} else { //got unexpected error
			return err
		}
	}
	return nil
}

func updateKey(o *Options, existingKey *config.KeyEntity) (*config.KeyEntity, error) {
	dt, err := config.NewDataType(o.DataType)
	if err != nil {
		return nil, err
	}
	existingKey.DataType = dt
	existingKey.Encrypted = o.Encrypted
	existingKey.Trigger = o.Trigger
	existingKey.Validator = o.Validator
	return o.Repository().CreateKey(existingKey)
}

func createKey(o *Options, key string) (*config.KeyEntity, error) {
	dt, err := config.NewDataType(o.DataType)
	if err != nil {
		return nil, err
	}
	return o.Repository().CreateKey(&config.KeyEntity{
		Key:       key,
		DataType:  dt,
		Encrypted: o.Encrypted,
		Validator: o.Validator,
		Trigger:   o.Trigger,
		Username:  "!TODO!", //FIXME
	})
}
