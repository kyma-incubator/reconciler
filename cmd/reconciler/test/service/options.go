package cmd

import (
	"fmt"
	reconCli "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
)

type Options struct {
	*reconCli.Options
	Version     string
	Namespace   string
	Component   string
	Profile     string
	InstallCRDs bool
}

func NewOptions(o *reconCli.Options) *Options {
	return &Options{
		o,
		"main",
		"kyma-system",
		"",
		"",
		false,
	}
}

func (o *Options) Validate() error {
	if o.Version == "" {
		return fmt.Errorf("version is undefined")
	}
	if o.Component == "" {
		return fmt.Errorf("component is undefined")
	}
	if o.Namespace == "" {
		return fmt.Errorf("namespace is undefined")
	}
	return nil
}
