package internal

import (
	"fmt"
	"time"
)

const (
	defaultMaxRetries = 3
	defaultRetryDelay = 1 * time.Second
)

type Config struct {
	MaxRetries int
	RetryDelay time.Duration
}

func (c *Config) validate() error {
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be < 0")
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaultMaxRetries
	}
	if c.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be < 0")
	}
	if c.RetryDelay == 0 {
		c.RetryDelay = defaultRetryDelay
	}
	return nil
}
