package compreconciler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/carlescere/scheduler"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
)

type Status string

const (
	Failed  Status = "failed"
	Error   Status = "error"
	Running Status = "running"
	Success Status = "success"
)

type StatusUpdater struct {
	job          *scheduler.Job
	maxFailures  int
	interval     int
	callbackURL  string
	status       Status
	failureCount int
}

func newStatusUpdater(interval int, callbackURL string, maxFailures int) *StatusUpdater {
	return &StatusUpdater{
		callbackURL: callbackURL,
		interval:    interval,
		status:      Running,
		maxFailures: maxFailures,
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
		if err != nil {
			log.Error(fmt.Sprintf("Status update request failed: %s", err))
			dumpResp, err := httputil.DumpResponse(resp, true)
			if err == nil {
				log.Error(fmt.Sprintf("Failed to dump response: %s", err))
			} else {
				log.Info(fmt.Sprintf("Response is: %s", string(dumpResp)))
			}
		}
	}
	job, err := scheduler.Every(su.interval).Seconds().Run(task)
	if err != nil {
		return err
	}
	su.job = job
	return nil
}

func (su *StatusUpdater) stop() {
	su.job.Quit <- true
	//important: the scheduler has to response with a valid response-code (e.g. 500/400 errors should lead to a retry of the call)
}

func (su *StatusUpdater) success() {
	su.status = Success
}

func (su *StatusUpdater) error() {
	su.status = Error
}

func (su *StatusUpdater) failed() {
	su.failureCount++
	if su.failureCount > su.maxFailures {
		su.error()
		su.stop()
	} else {
		su.status = Failed
	}
}
