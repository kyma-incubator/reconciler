package compreconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"go.uber.org/zap"

	"github.com/carlescere/scheduler"
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
	ctx        context.Context
	job        *scheduler.Job  //trigger for callback-handler to inform reconciler-controller
	interval   time.Duration   //interval for sending the latest status to reconciler-controller
	callback   CallbackHandler //callback-handler which trigger the callback logic to inform reconciler-controller
	status     Status          //current status
	maxRetries uint
	debug      bool
}

func newStatusUpdater(ctx context.Context, interval time.Duration,
	callback CallbackHandler, maxRetries uint, debug bool) *StatusUpdater {
	return &StatusUpdater{
		ctx:        ctx,
		interval:   interval,
		callback:   callback,
		maxRetries: maxRetries,
		debug:      debug,
		status:     NotStarted,
	}
}

func (su *StatusUpdater) logger() *zap.Logger {
	return newLogger(su.debug)
}

func (su *StatusUpdater) updateWithInterval(status Status) error {
	su.stopJob() //ensure previous scheduler-job is stopped before starting a new update routine

	task := func() {
		err := su.callback.Callback(status)
		if err == nil {
			su.logger().Debug(fmt.Sprintf("Interval-callback with status-update ('%s') finished successfully", status))
		} else {
			su.logger().Warn(fmt.Sprintf(
				"Interval-callback with status-update ('%s') to reconciler-controller failed: %s", status, err))
		}
	}

	var err error
	su.job, err = scheduler.Every(int(su.interval.Seconds())).Seconds().Run(task)
	return err
}

func (su *StatusUpdater) updateWithRetry(status Status) error {
	su.stopJob() //ensure scheduler-job is stopped before starting a new update routine

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

	return nil
}

func (su *StatusUpdater) CurrentStatus() Status {
	return su.status
}

func (su *StatusUpdater) Start() {
	go func(shutdownFct func()) {
		<-su.ctx.Done()
		su.logger().Info("Execution context closing: shutdown status updater")
		shutdownFct()
	}(su.stopJob)
}

func (su *StatusUpdater) stopJob() {
	if su.job != nil {
		su.job.Quit <- true
	}
}

func (su *StatusUpdater) Running() error {
	if err := su.statusChangeAllowed(Running); err != nil {
		return err
	}
	err := su.updateWithInterval(Running) //Running is an interim status: use interval to send heartbeat-request to reconciler-controller
	if err == nil {
		su.status = Running
	}
	return err
}

func (su *StatusUpdater) Success() error {
	if err := su.statusChangeAllowed(Success); err != nil {
		return err
	}
	err := su.updateWithRetry(Success) //Success is a final status: use retry because heartbeat-requests are no longer needed
	if err == nil {
		su.status = Success
	}
	return err
}

func (su *StatusUpdater) Error() error {
	if err := su.statusChangeAllowed(Error); err != nil {
		return err
	}
	err := su.updateWithRetry(Error) //Error is a final status: use retry because heartbeat-requests are no longer needed
	if err == nil {
		su.status = Error
	}
	return err
}

//Failed will send interval updates the reconcile-controller with status 'failed'
func (su *StatusUpdater) Failed() error {
	if err := su.statusChangeAllowed(Failed); err != nil {
		return err
	}
	err := su.updateWithInterval(Failed) //Failed is an interim status: use interval to send heartbeat-request to reconciler-controller
	if err == nil {
		su.status = Failed
	}
	return err
}

func (su *StatusUpdater) statusChangeAllowed(status Status) error {
	if su.status == Error || su.status == Success {
		return fmt.Errorf("cannot switch in '%s' status because we are already in final status '%s'", status, su.status)
	}
	return nil
}
