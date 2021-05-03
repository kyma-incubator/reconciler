package cmd

import (
	"os"
	"sort"
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
		Aliases: []string{"buckets", "bu"},
		Short:   "Get configuration key.",
		Long:    `List configuration keys or get a key.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(o, args)
		},
	}
	return cmd
}

func Run(o *getCmd.Options, keyFilter []string) error {
	if err := o.Validate(); err != nil {
		return err
	}

	allKeys, err := o.Repository().Keys()
	if err != nil {
		return err
	}

	//if filter is defined, show buckets with content
	if len(keyFilter) > 0 {
		filteredKeys := filterKeys(allKeys, keyFilter)
		return renderKeysWithValues(o, filteredKeys)
	}

	//otherwise show just the bucket itself
	return renderKeys(o, allKeys)
}

func filterKeys(allKeys []*config.KeyEntity, keyFilter []string) []*config.KeyEntity {
	//to improve speed, use map from bucket-name to bucket-entity
	keyByName := make(map[string]*config.KeyEntity, len(keyFilter))
	for _, key := range allKeys {
		keyByName[key.Key] = key
	}

	//filter keys
	filteredKeys := []*config.KeyEntity{}
	sort.Strings(keyFilter) //ensure the filtered keys are added to result in alphabetical order
	for _, filter := range keyFilter {
		if key, ok := keyByName[filter]; ok {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return filteredKeys
}

func renderKeys(o *getCmd.Options, keys []*config.KeyEntity) error {
	formatter, err := cli.NewOutputFormatter(o.OutputFormat)
	if err != nil {
		return err
	}

	if err := formatter.Header("Key", "Data Type", "Encrypted", "Created by", "Created at (UTC)", "Validation", "Trigger"); err != nil {
		return err
	}
	for _, key := range keys {
		if err := formatter.AddRow(key.Key, key.DataType, key.Encrypted, key.Username, key.Created.Format(time.RFC822Z), key.Validator, key.Trigger); err != nil {
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

	if err := formatter.Header("Key", "Data Type", "Encrypted", "Created by", "Created at (UTC)", "Values"); err != nil {
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
		if err := formatter.AddRow(key.Key, key.DataType, key.Encrypted, key.Username, key.Created.Format(time.RFC822Z), kvPairs); err != nil {
			return err
		}
	}
	return formatter.Output(os.Stdout)
}
