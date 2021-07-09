package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/internal/cli"
)

type Options struct {
	*cli.Options
	DataType  string
	Encrypted bool
	Validator string
	Trigger   string
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o, "", false, "", ""}
}

func (o *Options) Validate() error {
	if o.DataType == "" {
		return fmt.Errorf("Data type has to be specified")
	}
	return nil
}
