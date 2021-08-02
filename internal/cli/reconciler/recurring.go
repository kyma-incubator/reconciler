package reconciler

import (
	"fmt"
	"time"
)

type RecurringTaskConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

func (c *RecurringTaskConfig) validate() error {
	if c.Interval <= 0 {
		return fmt.Errorf("interval cannot be <= 0")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout cannot be <= 0")
	}
	return nil
}
