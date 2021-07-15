package compreconciler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/carlescere/scheduler"
	"github.com/kyma-incubator/reconciler/pkg/logger"
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
	job          *scheduler.Job //trigger for http-calls to reconciler-controller
	interval     time.Duration  //interval for sending the latest status to reconciler-controller
	callbackURL  string         //URL of the reconciler-controller
	status       Status         //current status
	lastUpdate   time.Time      //time when the last status update was successfully send to reconciler-controller
	retryTimeout time.Duration  //timeout until the status updater will stop retrying to send updates to the reconciler
}

func newStatusUpdater(interval time.Duration, callbackURL string, retryTimeout time.Duration) *StatusUpdater {
	return &StatusUpdater{
		callbackURL:  callbackURL,
		interval:     interval,
		status:       NotStarted,
		retryTimeout: retryTimeout,
	}
}

func (su *StatusUpdater) start() error {
	task := func() {
		log, err := logger.NewLogger(true)
		if err != nil {
			log = zap.NewNop()
		}

		requestBody, err := json.Marshal(map[string]string{
			"status": string(su.status),
		})
		if err != nil {
			log.Error(err.Error())
		}

		resp, err := http.Post(su.callbackURL, "application/json", bytes.NewBuffer(requestBody))
		if err == nil {
			su.lastUpdate = time.Now()
		} else {
			log.Error(fmt.Sprintf("Status update request failed: %s", err))
			//dump request
			dumpResp, err := httputil.DumpResponse(resp, true)
			if err == nil {
				log.Error(fmt.Sprintf("Failed to dump response: %s", err))
			} else {
				log.Info(fmt.Sprintf("Response is: %s", string(dumpResp)))
			}
		}

		if su.stopStatusUpdates(err) {
			su.job.Quit <- true
		}
	}

	job, err := scheduler.Every(int(su.interval.Seconds())).Seconds().Run(task)
	if err != nil {
		return err
	}
	su.job = job
	su.status = Running

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

func (su *StatusUpdater) Running() bool {
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
