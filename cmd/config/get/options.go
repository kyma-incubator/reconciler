package cmd

import (
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/internal/cli"
)

type Options struct {
	*cli.Options
	OutputFormat string
	History      bool
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o, "", false}
}

func (o *Options) Validate() error {
	for _, supportedFormat := range cli.SupportedOutputFormats {
		if supportedFormat == o.OutputFormat {
			return nil
		}
	}
	return fmt.Errorf("Output format '%s' not supported - choose between '%s'", o.OutputFormat, strings.Join(cli.SupportedOutputFormats, "', '"))
}
