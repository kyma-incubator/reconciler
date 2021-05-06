package cmd

import (
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
