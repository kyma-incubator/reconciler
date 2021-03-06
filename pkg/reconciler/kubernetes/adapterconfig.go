package kubernetes

import (
	"fmt"
	"time"
)

const (
	progressTrackerInterval = 5 * time.Second
	progressTrackerTimeout  = 2 * time.Minute
	maxRetries              = 10
	retryDelay              = 1 * time.Second
)

type Config struct {
	ProgressInterval time.Duration
	ProgressTimeout  time.Duration
	MaxRetries       int
	RetryDelay       time.Duration
}

func (c *Config) validate() error {

	switch {
	case c.MaxRetries < 0:
		return fmt.Errorf("config MaxRetries cannot be < 0 (got %d)", c.MaxRetries)
	case c.RetryDelay < 0:
		return fmt.Errorf("config RetryDelay cannot be < 0 (got %d)", c.RetryDelay)
	case c.ProgressInterval < 0:
		return fmt.Errorf("config ProgressInterval cannot be < 0 (got %d)", c.ProgressInterval)
	case c.ProgressTimeout < 0:
		return fmt.Errorf("config ProgressTimeout cannot be < 0 (got %d)", c.ProgressTimeout)
	}

	if c.MaxRetries == 0 {
		c.MaxRetries = maxRetries
	}
	if c.RetryDelay == 0 {
		c.RetryDelay = retryDelay
	}
	if c.ProgressInterval == 0 {
		c.ProgressInterval = progressTrackerInterval
	}
	if c.ProgressTimeout == 0 {
		c.ProgressTimeout = progressTrackerTimeout
	}
	return nil
}
