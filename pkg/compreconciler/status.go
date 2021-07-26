package compreconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"

	"github.com/avast/retry-go"
	"go.uber.org/zap"
)

type Status string

const (
	NotStarted Status = "notstarted"
	Failed     Status = "failed"
	Error      Status = "error"
	Running    Status = "running"
	Success    Status = "success"
)

const (
	defaultStatusUpdaterInterval   = 30 * time.Second
	defaultStatusUpdaterMaxRetries = 5
	defaultStatusUpdaterRetryDelay = 30 * time.Second
)

type StatusUpdaterConfig struct {
	Interval   time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

func (su *StatusUpdaterConfig) validate() error {
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

type StatusUpdater struct {
	ctx             context.Context
	restartInterval chan bool       //trigger for callback-handler to inform reconciler-controller
	callback        CallbackHandler //callback-handler which trigger the callback logic to inform reconciler-controller
	status          Status          //current status
	debug           bool
	interrupted     bool //indicate whether the process was interrupted from outside
	config          StatusUpdaterConfig
}

func newStatusUpdater(ctx context.Context, callback CallbackHandler, debug bool, config StatusUpdaterConfig) (*StatusUpdater, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	return &StatusUpdater{
		ctx:             ctx,
		config:          config,
		restartInterval: make(chan bool),
		callback:        callback,
		debug:           debug,
		status:          NotStarted,
	}, nil
}

func (su *StatusUpdater) logger() *zap.Logger {
	return logger.NewOptionalLogger(su.debug)
}

func (su *StatusUpdater) updateWithInterval(status Status) {
	su.stopJob() //ensure previous interval-loop is stopped before starting a new loop

	task := func(status Status) {
		err := su.callback.Callback(status)
		if err == nil {
			su.logger().Debug(fmt.Sprintf("Interval-callback with status-update ('%s') finished successfully", status))
		} else {
			su.logger().Warn(fmt.Sprintf(
				"Interval-callback with status-update ('%s') to reconciler-controller failed: %s", status, err))
		}
	}

	go func(status Status, interval time.Duration) {
		su.logger().Debug(fmt.Sprintf("Starting new interval loop for status '%s'", status))
		task(status)
		for {
			select {
			case <-su.restartInterval:
				su.logger().Debug(fmt.Sprintf("Stop running interval loop for status '%s'", status))
				return
			case <-su.ctx.Done():
				su.logger().Debug(fmt.Sprintf("Stopping interval loop for status '%s' because context was closed", status))
				su.interrupted = true
				return
			case <-time.NewTicker(interval).C:
				su.logger().Debug(fmt.Sprintf("Interval loop for status '%s' executes callback", status))
				task(status)
			}
		}
	}(status, su.config.Interval)

	su.status = status
}

func (su *StatusUpdater) updateWithRetry(status Status) {
	su.stopJob() //ensure previous interval-loop is stopped before starting a new loop

	go func(ctx context.Context, s Status, retries int, delay time.Duration) {
		err := retry.Do(
			func() error {
				err := su.callback.Callback(s)
				if err == nil {
					su.logger().Debug(fmt.Sprintf("Retry-callback with status-update ('%s') finished successfully", status))
				} else {
					su.logger().Warn(fmt.Sprintf(
						"Retry-callback with status-update ('%s') to reconciler-controller failed: %s", status, err))
				}
				return err
			},
			retry.Context(ctx),
			retry.Attempts(uint(retries)),
			retry.Delay(delay),
			retry.LastErrorOnly(false))
		if err != nil {
			su.logger().Error(
				fmt.Sprintf("Retry-callback with status-update ('%s') failed: %s", status, err))
		}
	}(su.ctx, status, su.config.MaxRetries, su.config.RetryDelay)

	su.status = status
}

func (su *StatusUpdater) CurrentStatus() Status {
	return su.status
}

func (su *StatusUpdater) stopJob() {
	if su.status == Running || su.status == Failed {
		su.restartInterval <- true
	}
}

func (su *StatusUpdater) Running() error {
	if err := su.statusChangeAllowed(Running); err != nil {
		return err
	}
	su.updateWithInterval(Running) //Running is an interim status: use interval to send heartbeat-request to reconciler-controller
	return nil
}

func (su *StatusUpdater) Success() error {
	if err := su.statusChangeAllowed(Success); err != nil {
		return err
	}
	su.updateWithRetry(Success) //Success is a final status: use retry because heartbeat-requests are no longer needed
	return nil
}

func (su *StatusUpdater) Error() error {
	if err := su.statusChangeAllowed(Error); err != nil {
		return err
	}
	su.updateWithRetry(Error) //Error is a final status: use retry because heartbeat-requests are no longer needed
	return nil
}

//Failed will send interval updates the reconcile-controller with status 'failed'
func (su *StatusUpdater) Failed() error {
	if err := su.statusChangeAllowed(Failed); err != nil {
		return err
	}
	su.updateWithInterval(Failed) //Failed is an interim status: use interval to send heartbeat-request to reconciler-controller
	return nil
}

func (su *StatusUpdater) statusChangeAllowed(status Status) error {
	if su.interrupted {
		return fmt.Errorf("cannot change status to '%s' because status updater was interrupted", status)
	}
	if su.status == Error || su.status == Success {
		return fmt.Errorf("cannot switch in '%s' status because we are already in final status '%s'", status, su.status)
	}
	return nil
}
