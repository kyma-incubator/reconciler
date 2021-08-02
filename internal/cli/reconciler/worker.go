package reconciler

import (
	"fmt"
	"time"
)

type WorkerConfig struct {
	Workers int
	Timeout time.Duration
}

func (c *WorkerConfig) validate() error {
	if c.Workers <= 0 {
		return fmt.Errorf("workers cannot be set to < 0")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout for workers cannot be set to < 0")
	}
	return nil
}
