package compreconciler

import "time"

type Status string

const (
	Failed  Status = "failed"
	Error   Status = "error"
	Running Status = "running"
	Success Status = "success"
)

type StatusUpdater struct {
	maxFailures  int
	interval     time.Duration
	callbackUrl  string
	status       Status
	failureCount int
}

func newStatusUpdater(interval time.Duration, callbackUrl string, maxFailures int) *StatusUpdater {
	return &StatusUpdater{
		callbackUrl: callbackUrl,
		interval:    interval,
		status:      Running,
		maxFailures: maxFailures,
	}
}

func (su *StatusUpdater) start() {
	//send su.status each 30 seconds to callbackUrl
}

func (su *StatusUpdater) stop(result Status) {
	//send su.status each 30 seconds to callbackUrl

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
	} else {
		su.status = Failed
	}
}
