package progress

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"

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

type trackerResource struct {
	kind      WatchableResource
	name      string
	namespace string
	info      *resource.Info
}

func (o *trackerResource) String() string {
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
	objects  []*trackerResource
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
	inState, err := pt.allWatchableInState(ctx, targetState)
	if err != nil {
		pt.logger.Warnf("Failed to verify initial Kubernetes resource state: %v", err)
	}
	if inState {
		//we are already done
		pt.logger.Debugf("Watchable resources are already in target state '%s': no recurring checks triggered", targetState)
		return nil
	}

	//start verifying the installation status in an interval
	timer := time.NewTicker(pt.interval)
	timeout := time.After(pt.timeout)
	for {
		select {
		case <-timer.C:
			inState, err := pt.allWatchableInState(ctx, targetState)
			if err != nil {
				pt.logger.Warnf("Failed to check progress of resource transition to state '%s' "+
					"but will retry until timeout is reached: %s", targetState, err)
			}
			if inState {
				timer.Stop()
				pt.logger.Debugf("Watchable resources reached target state '%s'", targetState)
				return nil
			}
		case <-ctx.Done():
			pt.logger.Debugf("Stop checking progress of resource transition to state '%s' "+
				"because parent context got closed", targetState)
			pt.logWatchableResourcesAsInfo()
			return &e.ContextClosedError{
				Message: fmt.Sprintf("Running resource transition to state '%s' was not completed: "+
					"transition is treated as failed", targetState),
			}
		case <-timeout:
			err := fmt.Errorf("progress tracker reached timeout (%.0f secs): "+
				"stop checking progress of resource transition to state '%s'",
				pt.timeout.Seconds(), targetState)
			pt.logger.Warn(err.Error())
			pt.dumpWatchableResourcesAsInfo(ctx)
			return err
		}
	}
}

func (pt *Tracker) AddResource(kind WatchableResource, namespace, name string) {
	pt.objects = append(pt.objects, &trackerResource{
		kind:      kind,
		namespace: namespace,
		name:      name,
	})
}

func (pt *Tracker) AddResourceWithInfo(kind WatchableResource, namespace, name string, info *resource.Info) {
	pt.objects = append(pt.objects, &trackerResource{
		kind:      kind,
		namespace: namespace,
		name:      name,
		info:      info,
	})
}

func (pt *Tracker) allWatchableInState(ctx context.Context, targetState State) (bool, error) {
	switch targetState {
	case ReadyState:
		return pt.isInReadyState(ctx)
	case TerminatedState:
		return pt.isInTerminatedState(ctx)
	default:
		return false, fmt.Errorf("state '%s' not supported", targetState)
	}
}

func (pt *Tracker) isInReadyState(ctx context.Context) (bool, error) {
	for _, object := range pt.objects {
		var err error
		ready := true

		switch object.kind {
		case Pod:
			ready, err = isPodReady(ctx, pt.client, object)
		case Deployment:
			ready, err = isDeploymentReady(ctx, pt.client, object)
		case DaemonSet:
			ready, err = isDaemonSetReady(ctx, pt.client, object)
		case StatefulSet:
			ready, err = isStatefulSetReady(ctx, pt.client, object)
		case Job:
			ready, err = isJobReady(ctx, pt.client, object)
		case CustomResourceDefinition:
			if object.info == nil {
				return false, fmt.Errorf("please use AddResourceWithInfo instead of AddResource for progress tracking CRD resources")
			}
			ready, err = isCRDReady(ctx, object)
			if err != nil {
				ready, err = isCRDBetaReady(ctx, object)
			}
		}

		if err != nil {
			pt.logger.Errorf("Failed to get resource of %v: %s", object, err)
			return false, err
		}
		if !ready {
			pt.logger.Debugf("Transition of %s to ready state is still ongoing", object.name)
			return false, nil
		}
	}

	pt.logger.Debug("All resources are ready")
	return true, nil

}

func (pt *Tracker) isInTerminatedState(ctx context.Context) (bool, error) {
	for _, object := range pt.objects {
		var err error

		switch object.kind {
		case Pod:
			_, err = pt.client.CoreV1().Pods(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
		case Deployment:
			_, err = pt.client.AppsV1().Deployments(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
		case DaemonSet:
			_, err = pt.client.AppsV1().DaemonSets(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
		case StatefulSet:
			_, err = pt.client.AppsV1().StatefulSets(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
		case Job:
			_, err = pt.client.BatchV1().Jobs(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
		case CustomResourceDefinition:
			if object.info == nil {
				err = fmt.Errorf("please use AddResourceWithInfo instead of AddResource for progress tracking CRD resources")
			} else {
				err = object.info.Get()
			}
		}

		if err == nil {
			pt.logger.Debugf("Termination of %s is still ongoing", object.name)
			return false, nil
		}
		if !errors.IsNotFound(err) {
			pt.logger.Errorf("Failed to get resource %v: %s", object, err)
			return false, err
		}
	}

	pt.logger.Debug("All resources are terminated")
	return true, nil
}

func (pt Tracker) logWatchableResourcesAsInfo() {
	for _, rs := range pt.objects {
		pt.logger.Infof("Tracker stopped checking the progress of "+
			"the following resource: %v", rs)
	}
}

func (pt Tracker) dumpWatchableResourcesAsInfo(ctx context.Context) {
	for _, rs := range pt.objects {
		buf, err := pt.resourceJSON(ctx, rs)

		if err != nil {
			pt.logger.Errorf("Tracker stopped checking the progress of "+
				"the following resource: %v. Failed to get resource: %s", rs, err)
		}
		pt.logger.Infof("Tracker stopped checking the progress of "+
			"the following resource: %v. Resource state: %s", rs, buf)
	}
}

func (pt Tracker) resourceJSON(ctx context.Context, rs *trackerResource) ([]byte, error) {
	switch rs.kind {
	case Pod:
		r, err := pt.client.CoreV1().Pods(rs.namespace).Get(ctx, rs.name, metav1.GetOptions{})
		if err == nil {
			return json.Marshal(r)
		}
		return nil, err
	case Deployment:
		r, err := pt.client.AppsV1().Deployments(rs.namespace).Get(ctx, rs.name, metav1.GetOptions{})
		if err == nil {
			return json.Marshal(r)
		}
		return nil, err
	case DaemonSet:
		r, err := pt.client.AppsV1().DaemonSets(rs.namespace).Get(ctx, rs.name, metav1.GetOptions{})
		if err == nil {
			return json.Marshal(r)
		}
		return nil, err
	case StatefulSet:
		r, err := pt.client.AppsV1().StatefulSets(rs.namespace).Get(ctx, rs.name, metav1.GetOptions{})
		if err == nil {
			return json.Marshal(r)
		}
		return nil, err
	case Job:
		r, err := pt.client.BatchV1().Jobs(rs.namespace).Get(ctx, rs.name, metav1.GetOptions{})
		if err == nil {
			return json.Marshal(r)
		}
		return nil, err
	case CustomResourceDefinition:
		if rs.info == nil {
			return nil, fmt.Errorf("please use AddResourceWithInfo instead of AddResource for progress tracking CRD resources")
		} else {
			if err := rs.info.Get(); err != nil {
				return nil, err
			}
			return json.Marshal(rs.info.Object)
		}
	default:
		return nil, fmt.Errorf("Resource type not supported: %s", rs.kind)
	}
}
