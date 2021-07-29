package chart

import (
	"fmt"
	"time"
)

const (
	defaultWorkersCount  = 4
	defaultCancelTimeout = 2 * time.Minute
	defaultQuitTimeout   = 3 * time.Minute
)

type Options struct {
	WorkersCount  int
	CancelTimeout time.Duration
	QuitTimeout   time.Duration
}

func (o *Options) validate() error {
	if o.WorkersCount < 0 {
		return fmt.Errorf("WorkersCount cannot be < 0")
	}
	if o.WorkersCount == 0 {
		o.WorkersCount = defaultWorkersCount
	}
	if o.CancelTimeout.Microseconds() == int64(0) {
		o.CancelTimeout = defaultCancelTimeout
	}
	if o.QuitTimeout.Microseconds() == int64(0) {
		o.QuitTimeout = defaultQuitTimeout
	}
	if o.QuitTimeout < o.CancelTimeout {
		return fmt.Errorf("Quit timeout is lower than cancel timeout")
	}
	return nil
}
