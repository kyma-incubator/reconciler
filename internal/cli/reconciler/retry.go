package reconciler

import (
	"fmt"
	"time"
)

type RetryConfig struct {
	MaxRetries int
	RetryDelay time.Duration
}

func (c *RetryConfig) validate() error {
	if c.MaxRetries <= 0 {
		return fmt.Errorf("max-retries cannot be <= 0")
	}
	if c.RetryDelay <= 0 {
		return fmt.Errorf("retry-delay cannot be <= 0")
	}
	return nil
}
