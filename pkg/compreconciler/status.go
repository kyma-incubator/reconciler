package compreconciler

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	logger "github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
	"time"

	"github.com/carlescere/scheduler"
)

type Status string

const (
	Failed  Status = "failed"
	Error   Status = "error"
	Running Status = "running"
	Success Status = "success"
	Stopped Status = "stopped"
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
	}
}
func (su *StatusUpdater) logger() *zap.Logger {
	logger, err := logger.NewLogger(su.debug)
	if err != nil {
		logger = zap.NewNop()
	}
	return logger
}

func (su *StatusUpdater) updateWithInterval(status Status) error {
	su.stop() //ensure scheduler-job is stopped before starting a new update routine

	task := func() {
		err := su.callback.Callback(status)
		su.logger().Warn(fmt.Sprintf(
			"Interval-callback with status-update ('%s') to reconciler-controller failed: %s", status, err))
	}

	var err error
	su.job, err = scheduler.Every(int(su.interval.Seconds())).Seconds().Run(task)
	return err
}

func (su *StatusUpdater) updateWithRetry(status Status) error {
	su.stop() //ensure scheduler-job is stopped before starting a new update routine

	go func(ctx context.Context, s Status, retries uint) {
		err := retry.Do(
			func() error {
				err := su.callback.Callback(s)
				if err != nil {
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
				fmt.Sprintf(" Reached max-retries for retry-callback with status-update ('%s'): %s", status, err))
		}
	}(su.ctx, status, su.maxRetries)

	return nil
}

func (su *StatusUpdater) CurrentStatus() Status {
	return su.status
}

func (su *StatusUpdater) Start() error {
	go func() { // watch parent context: if it gets closed, stop the status updater
		<-su.ctx.Done()
		su.stop()
	}()
	return su.Running()
}

func (su *StatusUpdater) stop() {
	if su.job != nil {
		su.job.Quit <- true
	}
	su.status = Stopped
}

func (su *StatusUpdater) Running() error {
	if err := su.statusChangeAllowed(Running); err != nil {
		return err
	}
	err := su.updateWithInterval(Running)
	if err == nil {
		su.status = Running
	}
	return err
}

func (su *StatusUpdater) Success() error {
	if err := su.statusChangeAllowed(Success); err != nil {
		return err
	}
	err := su.updateWithRetry(Success)
	if err == nil {
		su.status = Success
	}
	return err
}

func (su *StatusUpdater) Error() error {
	if err := su.statusChangeAllowed(Error); err != nil {
		return err
	}
	err := su.updateWithRetry(Error)
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
	err := su.updateWithInterval(Failed)
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
