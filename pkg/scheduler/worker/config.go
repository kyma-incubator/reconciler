package worker

import (
	"fmt"
	"time"
)

const (
	defaultMaxParallelOperations  = 25
	defaultPoolSize               = 500
	defaultOperationCheckInterval = 30 * time.Second
	defaultInvokerMaxRetries      = 5
	defaultInvokerRetryDelay      = 5 * time.Second
	defaultMaxOperationRetries    = 5
)

type Config struct {
	MaxParallelOperations  int //maximal parallel operations per cluster (respectively reconciliation)
	PoolSize               int
	OperationCheckInterval time.Duration
	InvokerMaxRetries      int
	InvokerRetryDelay      time.Duration
	MaxOperationRetries    int
}

func (c *Config) validate() error {
	if c.MaxParallelOperations < 0 {
		return fmt.Errorf("parallel operations per reconciliation cannot be < 0 (was %d)", c.MaxParallelOperations)
	}
	if c.MaxParallelOperations == 0 {
		c.MaxParallelOperations = defaultMaxParallelOperations
	}
	if c.PoolSize < 0 {
		return fmt.Errorf("pool size cannot be < 0 (was %d)", c.PoolSize)
	}
	if c.PoolSize == 0 {
		c.PoolSize = defaultPoolSize
	}
	if c.OperationCheckInterval < 0 {
		return fmt.Errorf("interval cannot be < 0 (was %.1f sec)", c.OperationCheckInterval.Seconds())
	}
	if c.OperationCheckInterval == 0 {
		c.OperationCheckInterval = defaultOperationCheckInterval
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
	// do we need to throw an error when the config value is negative? why not set the default value in this case too?
	if c.MaxOperationRetries < 0 {
		return fmt.Errorf("invoker max operations retries cannot be < 0 (was %d)", c.InvokerRetryDelay.Seconds())
	}
	if c.MaxOperationRetries == 0 {
		c.MaxOperationRetries = defaultMaxOperationRetries
	}
	return nil
}
