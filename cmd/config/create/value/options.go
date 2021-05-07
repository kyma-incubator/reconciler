package cmd

import (
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
