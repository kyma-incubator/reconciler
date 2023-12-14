package config

import (
	"fmt"

	"github.com/pkg/errors"
)

const FallbackComponentReconciler = "base"

type ComponentCRD struct {
	Group   string
	Version string
	Kind    string
}

type ComponentReconciler struct {
	URL string
}

type SchedulerConfig struct {
	PreComponents  [][]string
	Reconcilers    map[string]ComponentReconciler
	DeleteStrategy string
	ComponentCRDs  map[string]ComponentCRD
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
	return nil
}
