package reconciler

import (
	"github.com/kyma-incubator/reconciler/internal/cli"
)

type Options struct {
	*cli.Options
	Workspace             string
	ServerConfig          *ServerConfig
	WorkerConfig          *WorkerConfig
	RetryConfig           *RetryConfig
	StatusUpdaterConfig   *RecurringTaskConfig
	ProgressTrackerConfig *RecurringTaskConfig
}

func NewOptions(o *cli.Options) *Options {
	return &Options{
		o,
		".",
		&ServerConfig{},
		&WorkerConfig{},
		&RetryConfig{},
		&RecurringTaskConfig{},
		&RecurringTaskConfig{},
	}
}

func (o *Options) Validate() error {
	if o.Workspace == "" {
		o.Workspace = "."
	}
	if err := o.ServerConfig.validate(); err != nil {
		return err
	}
	if err := o.WorkerConfig.validate(); err != nil {
		return err
	}
	if err := o.RetryConfig.validate(); err != nil {
		return err
	}
	if err := o.StatusUpdaterConfig.validate(); err != nil {
		return err
	}
	if err := o.ProgressTrackerConfig.validate(); err != nil {
		return err
	}
	return nil
}
