package cli

import "go.uber.org/zap"

type Options struct {
	Verbose        bool
	NonInteractive bool
	Logger         *zap.Logger
}
