package progress

import (
	"context"
	"fmt"
	"time"

	e "github.com/kyma-incubator/reconciler/pkg/error"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultProgressInterval = 20 * time.Second
	defaultProgressTimeout  = 10 * time.Minute

	ReadyState      State = "ready"
	TerminatedState State = "terminated"
)

type State string

type resource struct {
	kind      WatchableResource
	name      string
	namespace string
}

func (o *resource) String() string {
	return fmt.Sprintf("%s [namespace:%s|name:%s]", o.kind, o.namespace, o.name)
}

type Config struct {
	Interval time.Duration
	Timeout  time.Duration
}

func (ptc *Config) validate() error {
	if ptc.Interval < 0 {
		return fmt.Errorf("progress tracker status-check interval cannot be < 0")
	}
	if ptc.Interval == 0 {
		ptc.Interval = defaultProgressInterval
	}
	if ptc.Timeout < 0 {
		return fmt.Errorf("progress tracker timeout cannot be < 0")
	}
	if ptc.Timeout == 0 {
		ptc.Timeout = defaultProgressTimeout
	}
	if ptc.Timeout <= ptc.Interval {
		return fmt.Errorf("progress tracker will never run because configured timeout "+
			"is <= as the check interval :%.0f secs <= %.0f secs", ptc.Timeout.Seconds(), ptc.Interval.Seconds())
	}
	return nil
}

type Tracker struct {
	objects  []*resource
	client   kubernetes.Interface
	interval time.Duration
	timeout  time.Duration
	logger   *zap.SugaredLogger
}

func NewProgressTracker(client kubernetes.Interface, logger *zap.SugaredLogger, config Config) (*Tracker, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &Tracker{
		client:   client,
		interval: config.Interval,
		timeout:  config.Timeout,
		logger:   logger,
	}, nil
}

func (pt *Tracker) Watch(ctx context.Context, targetState State) error {
	if len(pt.objects) == 0 { //check if any watchable resources were added
		pt.logger.Debugf("No watchable resources defined: transition to state '%s' "+
			"will be treated as successfully finished", targetState)
		return nil
	}

	//initial installation status check
	ready, err := pt.isInState(ctx, targetState)
	if err != nil {
		pt.logger.Warnf("Failed to verify initial Kubernetes resource state: %v", err)
	}
	if ready {
		//we are already done
		pt.logger.Debugf("Watchable resources are already in target state '%s': no recurring checks triggered", targetState)
		return nil
	}

	//start verifying the installation status in an interval
	readyCheck := time.NewTicker(pt.interval)
	timeout := time.After(pt.timeout)
	for {
		select {
		case <-readyCheck.C:
			ready, err := pt.isInState(ctx, targetState)
			if err != nil {
				pt.logger.Warnf("Failed to check progress of resource transition to state '%s' "+
					"but will retry until timeout is reached: %s", targetState, err)
			}
			if ready {
				readyCheck.Stop()
				pt.logger.Debugf("Watchable resources reached target state '%s'", targetState)
				return nil
			}
		case <-ctx.Done():
			pt.logger.Debugf("Stop checking progress of resource transition to state '%s' "+
				"because parent context got closed", targetState)
			return &e.ContextClosedError{
				Message: fmt.Sprintf("Running resource transition to state '%s' was not completed: "+
					"transition is treated as failed", targetState),
			}
		case <-timeout:
			err := fmt.Errorf("progress tracker reached timeout (%.0f secs): "+
				"stop checking progress of resource transition to state '%s'",
				pt.timeout.Seconds(), targetState)
			pt.logger.Warn(err.Error())
			return err
		}
	}
}

func (pt *Tracker) AddResource(kind WatchableResource, namespace, name string) {
	pt.objects = append(pt.objects, &resource{
		kind:      kind,
		namespace: namespace,
		name:      name,
	})
}

func (pt *Tracker) isInState(ctx context.Context, targetState State) (bool, error) {
	var err error
	componentInState := true
	for _, object := range pt.objects {
		switch object.kind {
		case Pod:
			componentInState, err = podInState(ctx, pt.client, targetState, object)
		case Deployment:
			componentInState, err = deploymentInState(ctx, pt.client, targetState, object)
		case DaemonSet:
			componentInState, err = daemonSetInState(ctx, pt.client, targetState, object)
		case StatefulSet:
			componentInState, err = statefulSetInState(ctx, pt.client, targetState, object)
		case Job:
			componentInState, err = jobInState(ctx, pt.client, targetState, object)
		}
		pt.logger.Debugf("%s resource '%s:%s' is in state '%s': %t",
			object.kind, object.name, object.namespace, targetState, componentInState)
		if err != nil {
			pt.logger.Errorf("Failed to retrieve state of %v: %s", object, err)
			return false, err
		}
		if !componentInState { //at least one component is not ready
			pt.logger.Debugf("Resource transition to state '%s' is still ongoing", targetState)
			return false, nil
		}
	}
	pt.logger.Debugf("Resource transition to state '%s' finished successfully", targetState)
	return componentInState, nil
}
