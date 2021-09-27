package worker

import (
	"fmt"
	"time"
)

const (
	defaultPoolSize          = 500
	defaultInterval          = 30 * time.Second
	defaultInvokerMaxRetries = 5
	defaultInvokerRetryDelay = 5 * time.Second
)

type Config struct {
	PoolSize          int
	Interval          time.Duration
	InvokerMaxRetries int
	InvokerRetryDelay time.Duration
}

func (c *Config) validate() error {
	if c.PoolSize < 0 {
		return fmt.Errorf("pool size cannot be < 0 (was %d)", c.PoolSize)
	}
	if c.PoolSize == 0 {
		c.PoolSize = defaultPoolSize
	}
	if c.Interval < 0 {
		return fmt.Errorf("interval cannot be < 0 (was %.1f sec)", c.Interval.Seconds())
	}
	if c.Interval == 0 {
		c.Interval = defaultInterval
	}
	if c.InvokerMaxRetries < 0 {
		return fmt.Errorf("invoker retries cannot be < 0 (was %d)", c.InvokerMaxRetries)
	}
	if c.InvokerMaxRetries == 0 {
		c.InvokerMaxRetries = defaultInvokerMaxRetries
	}
	if c.InvokerRetryDelay < 0 {
		return fmt.Errorf("invoker retry delay cannot be < 0 (was %.1f sec)", c.InvokerRetryDelay.Seconds())
	}
	if c.InvokerRetryDelay == 0 {
		c.InvokerRetryDelay = defaultInvokerRetryDelay
	}
	return nil
}
