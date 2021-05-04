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
		Use:     "key",
		Aliases: []string{"keys", "k"},
		Short:   "Get configuration key.",
		Long:    `List configuration keys or get a key.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(o, args)
		},
	}
	cmd.Flags().BoolVar(&o.History, "history", false, "Show history of a configuration key")
	return cmd
}

func Run(o *getCmd.Options, keyFilter []string) error {
	if err := o.Validate(); err != nil {
		return err
	}

	keysProcessor, err := newKeysProcessor(o.Repository())
	if err != nil {
		return err
	}

	keysProcessor.filter(keyFilter)
	if o.History {
		keysProcessor.withHistory()
	}
	keys, err := keysProcessor.get()
	if err != nil {
		return err
	}

	if len(keyFilter) > 0 {
		// render with values if particular keys were selected by user
		return renderKeysWithValues(o, keys)
	}

	// render all keys (without values)
	return renderKeys(o, keys)
}

func renderKeys(o *getCmd.Options, keys []*config.KeyEntity) error {
	formatter, err := cli.NewOutputFormatter(o.OutputFormat)
	if err != nil {
		return err
	}

	if err := formatter.Header("Key", "Data Type", "Encrypted", "Created by",
		"Created at (UTC)", "Validation", "Trigger", "Version"); err != nil {
		return err
	}
	for _, key := range keys {
		if err := formatter.AddRow(key.Key, key.DataType, key.Encrypted, key.Username,
			key.Created.Format(time.RFC822Z), key.Validator, key.Trigger, key.Version); err != nil {
			return err
		}
	}
	return formatter.Output(os.Stdout)
}

func renderKeysWithValues(o *getCmd.Options, keys []*config.KeyEntity) error {
	formatter, err := cli.NewOutputFormatter(o.OutputFormat)
	if err != nil {
		return err
	}

	if err := formatter.Header("Key", "Data Type", "Encrypted", "Created by",
		"Created at (UTC)", "Validation", "Trigger", "Version", "Values"); err != nil {
		return err
	}
	for _, key := range keys {
		values, err := o.Repository().ValuesByKey(key)
		if err != nil {
			return err
		}
		kvPairs := make(map[string][]interface{}, len(values))
		for _, value := range values {
			if _, ok := kvPairs[value.Bucket]; !ok {
				kvPairs[value.Bucket] = []interface{}{}
			}
			kvPairs[value.Bucket] = append(kvPairs[value.Bucket], value.Value)
		}
		if err := formatter.AddRow(key.Key, key.DataType, key.Encrypted, key.Username,
			key.Created.Format(time.RFC822Z), key.Validator, key.Trigger, key.Version, kvPairs); err != nil {
			return err
		}
	}
	return formatter.Output(os.Stdout)
}
