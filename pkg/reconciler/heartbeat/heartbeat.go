package heartbeat

import (
	"context"
	"fmt"
	"sync"
	"time"

	e "github.com/kyma-incubator/reconciler/pkg/error"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	cb "github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"go.uber.org/zap"
)

const (
	defaultHeartbeatSenderInterval = 30 * time.Second
	defaultHeartbeatSenderTimeout  = 1 * time.Hour
)

type Config struct {
	Interval time.Duration
	Timeout  time.Duration
}

func (su *Config) validate() error {
	if su.Interval < 0 {
		return fmt.Errorf("heartbeat interval cannot be < 0 but was %.1f secs", su.Interval.Seconds())
	}
	if su.Interval == 0 {
		su.Interval = defaultHeartbeatSenderInterval
	}
	if su.Timeout < 0 {
		return fmt.Errorf("timeout cannot be < 0 but was %d", su.Timeout)
	}
	if su.Timeout == 0 {
		su.Timeout = defaultHeartbeatSenderTimeout
	}

	if su.Timeout <= su.Interval {
		return fmt.Errorf("timeout cannot be <= interval (%.1f secs <= %.1f secs)",
			su.Timeout.Seconds(), su.Interval.Seconds())
	}
	return nil
}

type Sender struct {
	ctx             context.Context
	ctxClosed       bool //indicate whether the process was interrupted by parent context
	config          Config
	status          reconciler.Status //current status
	callback        cb.Handler        //callback-handler which trigger the callback logic to inform reconciler-controller
	restartInterval chan bool         //trigger for callback-handler to inform reconciler-controller
	m               sync.Mutex
	logger          *zap.SugaredLogger
}

func NewHeartbeatSender(ctx context.Context, callback cb.Handler, logger *zap.SugaredLogger, config Config) (*Sender, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &Sender{
		ctx:             ctx,
		config:          config,
		restartInterval: make(chan bool),
		callback:        callback,
		status:          reconciler.StatusNotstarted,
		logger:          logger,
	}, nil
}

func (su *Sender) closeContext() {
	su.m.Lock()
	defer su.m.Unlock()
	su.ctxClosed = true
}

func (su *Sender) isContextClosed() bool {
	su.m.Lock()
	defer su.m.Unlock()
	return su.ctxClosed
}

func (su *Sender) sendUpdate(status reconciler.Status, reason error, onlyOnce bool) {
	su.stopJob() //ensure previous interval-loop is stopped before starting a new loop

	task := func(status reconciler.Status, rootCause error) error {
		err := su.callback.Callback(&reconciler.CallbackMessage{
			Status: status,
			Error: func(err error) string {
				if err != nil {
					return err.Error()
				}
				return ""
			}(rootCause),
		})
		if err == nil {
			su.logger.Debugf("Heartbeat communicated status '%s' successfully to mothership-reconciler", status)
		} else {
			su.logger.Warnf("Heartbeat failed to communicate status update '%s' "+
				"to mothership-reconciler: %s", status, err)
		}
		return err
	}

	go func(status reconciler.Status, rootCause error, interval time.Duration, timeout time.Duration, onlyOnce bool) {
		su.logger.Debugf("Heartbeat starts sending status '%s'", status)
		if err := task(status, rootCause); err == nil && onlyOnce {
			return
		}

		for {
			select {
			case <-su.restartInterval:
				su.logger.Debugf("Heartbeat stops sending status '%s'", status)
				return
			case <-su.ctx.Done():
				su.closeContext()

				//send error resonse
				var reconcilerStatus reconciler.Status
				if su.ctx.Err() == context.DeadlineExceeded { //operation not finished within given time range: error!
					reconcilerStatus = reconciler.StatusError
					su.logger.Warnf("Heartbeat context got closed caused by timeout: sending status '%s'",
						reconcilerStatus)
				} else {
					reconcilerStatus = reconciler.StatusFailed
					su.logger.Infof("Heartbeat context got closed by parent context: sending status '%s'",
						reconcilerStatus)
				}

				//try to send status before interval starts (to avoid waiting period until first interval tick is reached)
				if err := task(reconcilerStatus, su.ctx.Err()); err == nil {
					return
				}

				//error could not be send, retry in loop
				ticker := time.NewTicker(interval)
				giveUp := time.NewTimer(timeout)
				for {
					select {
					case <-ticker.C:
						if err := task(reconcilerStatus, su.ctx.Err()); err == nil {
							return
						}
					case <-giveUp.C:
						su.logger.Errorf("Heartbeat failed to communicated status '%s' after context got closed "+
							"(ctx error: %s): timeout occcurred", reconcilerStatus, su.ctx.Err())
						return
					}
				}
			case <-time.NewTicker(interval).C:
				err := task(status, rootCause)
				if err != nil {
					su.logger.Warnf("Heartbeat failed to communicate status '%s' "+
						"but will retry: %s", status, err)
				} else if onlyOnce {
					su.logger.Debugf("Hearbeat communicated status '%s' successfully after retry: "+
						"stopping update loop", status)
					return
				}
			}
		}
	}(status, reason, su.config.Interval, su.config.Timeout, onlyOnce)

	su.status = status
}

func (su *Sender) CurrentStatus() reconciler.Status {
	return su.status
}

func (su *Sender) stopJob() {
	if su.status == reconciler.StatusRunning || su.status == reconciler.StatusFailed {
		su.restartInterval <- true
	}
}

func (su *Sender) Running() error {
	if err := su.statusChangeAllowed(reconciler.StatusRunning); err != nil {
		return err
	}
	su.sendUpdate(reconciler.StatusRunning, nil, false) //Running is an interim status: use interval to send heartbeat-request to reconciler-controller
	return nil
}

func (su *Sender) Failed(err error) error {
	if err := su.statusChangeAllowed(reconciler.StatusFailed); err != nil {
		return err
	}
	su.sendUpdate(reconciler.StatusFailed, err, false) //Failed is an interim status: use interval to send heartbeat-request to reconciler-controller
	return nil
}

func (su *Sender) Success() error {
	if err := su.statusChangeAllowed(reconciler.StatusSuccess); err != nil {
		return err
	}
	su.sendUpdate(reconciler.StatusSuccess, nil, true) //Success is a final status: use retry because heartbeat-requests are no longer needed
	return nil
}

func (su *Sender) Error(err error) error {
	if err := su.statusChangeAllowed(reconciler.StatusError); err != nil {
		return err
	}
	su.sendUpdate(reconciler.StatusError, err, true) //Error is a final status: use retry because heartbeat-requests are no longer needed
	return nil
}

func (su *Sender) statusChangeAllowed(status reconciler.Status) error {
	if su.isContextClosed() {
		return &e.ContextClosedError{
			Message: fmt.Sprintf("Cannot change status to '%s' because context of heartbeat sender is closed", status),
		}
	}
	if su.status == reconciler.StatusError || su.status == reconciler.StatusSuccess {
		return fmt.Errorf("cannot switch in '%s' status because we are already in final status '%s'", status, su.status)
	}
	return nil
}
