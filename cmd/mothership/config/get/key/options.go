package cmd

import (
	"github.com/kyma-incubator/reconciler/internal/cli"
)

type Options struct {
	*cli.Options
	History bool
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o, false}
}
