package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/internal/cli"
)

type Options struct {
	*cli.Options
	History    bool
	Key        string
	KeyVersion int64
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o, false, "", 0}
}

func (o *Options) Validate() error {
	if err := o.Options.Validate(); err != nil {
		return err
	}
	if o.Key == "" && o.KeyVersion <= 0 {
		return fmt.Errorf("Either key or key-version has to be specified")
	}
	return nil
}
