package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/internal/cli"
)

type Options struct {
	*cli.Options
	Key        string
	KeyVersion int64
	Bucket     string
	Value      string
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o, "", 0, "", ""}
}

func (o *Options) Validate() error {
	if err := o.Options.Validate(); err != nil {
		return err
	}
	if o.Bucket == "" {
		return fmt.Errorf("Bucket has to be specified")
	}
	if o.Key == "" && o.KeyVersion <= 0 {
		return fmt.Errorf("Either the key or the key-version has to be provided")
	}
	return nil
}
