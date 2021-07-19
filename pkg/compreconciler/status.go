package compreconciler

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"time"

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

type StatusUpdater struct {
	ctx             context.Context
	restartInterval chan bool       //trigger for callback-handler to inform reconciler-controller
	interval        time.Duration   //interval for sending the latest status to reconciler-controller
	callback        CallbackHandler //callback-handler which trigger the callback logic to inform reconciler-controller
	status          Status          //current status
	maxRetries      uint
	debug           bool
}

func newStatusUpdater(ctx context.Context, interval time.Duration,
	callback CallbackHandler, maxRetries uint, debug bool) *StatusUpdater {
	return &StatusUpdater{
		ctx:             ctx,
		interval:        interval,
		restartInterval: make(chan bool),
		callback:        callback,
		maxRetries:      maxRetries,
		debug:           debug,
		status:          NotStarted,
	}
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

	go func(status Status) {
		su.logger().Debug(fmt.Sprintf("Starting new interval loop for status '%s'", status))
		task(status)
		for {
			select {
			case <-su.restartInterval:
				su.logger().Debug(fmt.Sprintf("Stop running interval loop for status '%s'", status))
				return
			case <-su.ctx.Done():
				su.logger().Debug(fmt.Sprintf("Stopping interval loop for status '%s' because context was closed", status))
				return
			case <-time.NewTicker(su.interval).C:
				su.logger().Debug(fmt.Sprintf("Interval loop for status '%s' executes callback", status))
				task(status)
			}
		}
	}(status)

	su.status = status
}

func (su *StatusUpdater) updateWithRetry(status Status) {
	su.stopJob() //ensure previous interval-loop is stopped before starting a new loop

	go func(ctx context.Context, s Status, retries uint) {
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
			retry.Attempts(retries),
			retry.LastErrorOnly(false))
		if err != nil {
			su.logger().Error(
				fmt.Sprintf("Retry-callback with status-update ('%s') failed: %s", status, err))
		}
	}(su.ctx, status, su.maxRetries)

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
	if su.status == Error || su.status == Success {
		return fmt.Errorf("cannot switch in '%s' status because we are already in final status '%s'", status, su.status)
	}
	return nil
}
