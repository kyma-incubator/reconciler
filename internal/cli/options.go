package cli

import (
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/app"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
)

type Options struct {
	Verbose        bool
	NonInteractive bool
	OutputFormat   string
	logger         *zap.Logger
	Registry       *app.ApplicationRegistry //will be initialized during CLI bootstrap in main.go
}

func (o *Options) String() string {
	return fmt.Sprintf("CLI options: verbose=%t non-interactive=%t",
		o.Verbose, o.NonInteractive)
}

func (o *Options) Validate() error {
	for _, supportedFormat := range SupportedOutputFormats {
		if supportedFormat == o.OutputFormat {
			return nil
		}
	}
	return fmt.Errorf("Output format '%s' not supported - choose between '%s'", o.OutputFormat, strings.Join(SupportedOutputFormats, "', '"))
}

func (o *Options) Logger() *zap.Logger {
	if o.logger == nil {
		logger, err := logger.NewLogger(o.Verbose)
		if err != nil {
			panic(err)
		}
		o.logger = logger
	}
	return o.logger
}
