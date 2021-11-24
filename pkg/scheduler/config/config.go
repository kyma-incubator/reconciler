package config

import (
	"fmt"
	"github.com/pkg/errors"
)

const FallbackComponentReconciler = "base"

type ComponentReconciler struct {
	URL string
}

type SchedulerConfig struct {
	PreComponents [][]string
	Reconcilers   map[string]ComponentReconciler
}

type Config struct {
	Scheme    string
	Host      string
	Port      int
	Scheduler SchedulerConfig
}

func (c *Config) Validate() error {
	if c.Scheme == "" {
		return errors.New("scheme of mothership reconciler is not configured")
	}
	if c.Host == "" {
		return errors.New("host of mothership reconciler is not configured")
	}
	if c.Port <= 0 {
		return fmt.Errorf("port of  mothership reconciler '%d' is not configured or invalid", c.Port)
	}
	if len(c.Scheduler.Reconcilers) == 0 {
		return errors.New("reconciler mapping for mothership scheduler is not configured")
	}
	if len(c.Scheduler.PreComponents) == 0 {
		return errors.New("pre-components for mothership scheduler are not configured")
	}
	return nil
}
