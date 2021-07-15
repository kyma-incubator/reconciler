package compreconciler

import (
	"context"
	"time"

	"github.com/carlescere/scheduler"
)

type Status string

const (
	Failed  Status = "failed"
	Error   Status = "error"
	Running Status = "running"
	Success Status = "success"
)

type StatusUpdater struct {
	job          *scheduler.Job  //trigger for http-calls to reconciler-controller
	interval     time.Duration   //interval for sending the latest status to reconciler-controller
	callback     CallbackHandler //URL of the reconciler-controller
	status       Status          //current status
	lastUpdate   time.Time       //time when the last status update was successfully send to reconciler-controller
	retryTimeout time.Duration   //timeout until the status updater will stop retrying to send updates to the reconciler
}

func newStatusUpdater(interval time.Duration, callback CallbackHandler, retryTimeout time.Duration) *StatusUpdater {
	return &StatusUpdater{
		interval:     interval,
		callback:     callback,
		status:       Running,
		retryTimeout: retryTimeout,
	}
}

func (su *StatusUpdater) Start(ctx context.Context) error {
	//trigger callback with status update in a defined interval
	stopUpdating := make(chan bool)
	task := func() {
		err := su.callback.Callback(su.status)
		if su.stopStatusUpdates(err) {
			stopUpdating <- true
		}
	}
	job, err := scheduler.Every(int(su.interval.Seconds())).Seconds().Run(task)
	if err != nil {
		return err
	}
	su.job = job

	//stop updating either when receiving the stopUpdating-event or if context gets closed
	go func() {
		select {
		case <-stopUpdating:
			su.Stop()
		case <-ctx.Done():
			su.Stop()
		}
	}()

	return nil
}

//stopStatusUpdates checks if no further updates should be send to reconciler-controller
//(either because an end-state or the retry-timeout was reached)
func (su *StatusUpdater) stopStatusUpdates(lastReqError error) bool {
	if time.Since(su.lastUpdate) > su.retryTimeout {
		return true
	}
	return lastReqError == nil && (su.status == Error || su.status == Success)
}

func (su *StatusUpdater) IsRunning() bool {
	if su.job != nil {
		return su.job.IsRunning()
	}
	return false
}

func (su *StatusUpdater) CurentStatus() Status {
	return su.status
}

func (su *StatusUpdater) Success() {
	su.status = Success
}

func (su *StatusUpdater) Error() {
	su.status = Error
}

func (su *StatusUpdater) Failed() {
	su.status = Failed
}

func (su *StatusUpdater) Stop() {
	if su.IsRunning() {
		su.job.Quit <- true
	}
}
