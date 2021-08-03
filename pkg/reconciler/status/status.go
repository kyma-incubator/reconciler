package status

import (
	"context"
	"fmt"
	"sync"
	"time"

	e "github.com/kyma-incubator/reconciler/pkg/error"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	cb "github.com/kyma-incubator/reconciler/pkg/reconciler/callback"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/logger"

	"go.uber.org/zap"
)

const (
	defaultStatusUpdaterInterval   = 30 * time.Second
	defaultStatusUpdaterMaxRetries = 5
	defaultStatusUpdaterRetryDelay = 30 * time.Second
)

type Config struct {
	Interval   time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

func (su *Config) validate() error {
	if su.Interval < 0 {
		return fmt.Errorf("status update interval cannot be < 0 but was %.1f secs", su.Interval.Seconds())
	}
	if su.Interval == 0 {
		su.Interval = defaultStatusUpdaterInterval
	}
	if su.MaxRetries < 0 {
		return fmt.Errorf("retries cannot be < 0 but was %d", su.MaxRetries)
	}
	if su.MaxRetries == 0 {
		su.MaxRetries = defaultStatusUpdaterMaxRetries
	}
	if su.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be < 0 but was %.1f", su.RetryDelay.Seconds())
	}
	if su.RetryDelay == 0 {
		su.RetryDelay = defaultStatusUpdaterRetryDelay
	}
	return nil
}

type Updater struct {
	ctx             context.Context
	ctxClosed       bool //indicate whether the process was interrupted by parent context
	debug           bool
	config          Config
	status          reconciler.Status //current status
	callback        cb.Handler        //callback-handler which trigger the callback logic to inform reconciler-controller
	restartInterval chan bool         //trigger for callback-handler to inform reconciler-controller
	m               sync.Mutex
}

func NewStatusUpdater(ctx context.Context, callback cb.Handler, debug bool, config Config) (*Updater, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	return &Updater{
		ctx:             ctx,
		config:          config,
		restartInterval: make(chan bool),
		callback:        callback,
		debug:           debug,
		status:          reconciler.NotStarted,
	}, nil
}

func (su *Updater) closeContext() {
	su.m.Lock()
	defer su.m.Unlock()
	su.ctxClosed = true
}

func (su *Updater) isContextClosed() bool {
	su.m.Lock()
	defer su.m.Unlock()
	return su.ctxClosed
}

func (su *Updater) logger() *zap.SugaredLogger {
	return logger.NewOptionalLogger()
}

func (su *Updater) sendUpdate(status reconciler.Status, onlyOnce bool) {
	su.stopJob() //ensure previous interval-loop is stopped before starting a new loop

	task := func(status reconciler.Status) error {
		err := su.callback.Callback(status)
		if err == nil {
			su.logger().Debugf("Interval-callback with status-update ('%s') finished successfully", status)
		} else {
			su.logger().Warnf("Interval-callback with status-update ('%s') to reconciler-controller failed: %s", status, err)
		}
		return err
	}

	go func(status reconciler.Status, interval time.Duration, onlyOnce bool) {
		su.logger().Debugf("Starting new update loop for status '%s' (update only once: %t)", status, onlyOnce)
		if err := task(status); err == nil && onlyOnce {
			su.logger().Debugf("Status '%s' successfully communicated: stopping update loop", status)
			return
		}
		for {
			select {
			case <-su.restartInterval:
				su.logger().Debugf("Stop running update loop for status '%s'", status)
				return
			case <-su.ctx.Done():
				su.logger().Debugf("Stopping update loop for status '%s' because context was closed", status)
				su.closeContext()
				return
			case <-time.NewTicker(interval).C:
				su.logger().Debugf("Update loop for status '%s' executes callback", status)
				if err := task(status); err == nil && onlyOnce {
					su.logger().Debugf("Status '%s' successfully communicated after retry: stopping update loop", status)
					return
				}
			}
		}
	}(status, su.config.Interval, onlyOnce)

	su.status = status
}

func (su *Updater) CurrentStatus() reconciler.Status {
	return su.status
}

func (su *Updater) stopJob() {
	if su.status == reconciler.Running || su.status == reconciler.Failed {
		su.restartInterval <- true
	}
}

func (su *Updater) Running() error {
	if err := su.statusChangeAllowed(reconciler.Running); err != nil {
		return err
	}
	su.sendUpdate(reconciler.Running, false) //Running is an interim status: use interval to send heartbeat-request to reconciler-controller
	return nil
}

func (su *Updater) Success() error {
	if err := su.statusChangeAllowed(reconciler.Success); err != nil {
		return err
	}
	su.sendUpdate(reconciler.Success, true) //Success is a final status: use retry because heartbeat-requests are no longer needed
	return nil
}

func (su *Updater) Error() error {
	if err := su.statusChangeAllowed(reconciler.Error); err != nil {
		return err
	}
	su.sendUpdate(reconciler.Error, true) //Error is a final status: use retry because heartbeat-requests are no longer needed
	return nil
}

//Failed will send interval updates the reconcile-controller with status 'failed'
func (su *Updater) Failed() error {
	if err := su.statusChangeAllowed(reconciler.Failed); err != nil {
		return err
	}
	su.sendUpdate(reconciler.Failed, false) //Failed is an interim status: use interval to send heartbeat-request to reconciler-controller
	return nil
}

func (su *Updater) statusChangeAllowed(status reconciler.Status) error {
	if su.isContextClosed() {
		return &e.ContextClosedError{
			Message: fmt.Sprintf("Cannot change status to '%s' because context of status updater is closed", status),
		}
	}
	if su.status == reconciler.Error || su.status == reconciler.Success {
		return fmt.Errorf("cannot switch in '%s' status because we are already in final status '%s'", status, su.status)
	}
	return nil
}
