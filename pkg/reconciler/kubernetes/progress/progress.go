package progress

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"time"

	e "github.com/kyma-incubator/reconciler/pkg/error"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ready, err := pt.isInState(targetState)
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
			ready, err := pt.isInState(targetState)
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

func (pt *Tracker) isInState(targetState State) (bool, error) {
	var err error
	componentInState := true
	for _, object := range pt.objects {
		switch object.kind {
		case Pod:
			componentInState, err = pt.podInState(targetState, object)
		case Deployment:
			componentInState, err = pt.deploymentInState(targetState, object)
		case DaemonSet:
			componentInState, err = pt.daemonSetInState(targetState, object)
		case StatefulSet:
			componentInState, err = pt.statefulSetInState(targetState, object)
		case Job:
			componentInState, err = pt.jobInState(targetState, object)
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

func (pt *Tracker) deploymentInState(inState State, object *resource) (bool, error) {
	deploymentsClient := pt.client.AppsV1().Deployments(object.namespace)
	deployment, err := deploymentsClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	switch inState {
	case ReadyState:
		if err != nil {
			return false, err
		}

		if deployment.Spec.Paused {
			return false, nil
		}

		replicaSet, err := GetNewReplicaSet(deployment, pt.client.AppsV1())
		if err != nil || replicaSet == nil {
			return false, err
		}

		if !(replicaSet.Status.ReadyReplicas >= 1) {
			return false, err
		}
		return true, nil
	case TerminatedState:
		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	default:
		return false, fmt.Errorf("state '%s' not supported", inState)
	}
}

func (pt *Tracker) statefulSetInState(inState State, object *resource) (bool, error) {
	statefulSetClient := pt.client.AppsV1().StatefulSets(object.namespace)
	statefulSet, err := statefulSetClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	switch inState {
	case ReadyState:
		if err != nil {
			return false, err
		}

		if statefulSet.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
			return true, nil
		}

		var partition int
		var replicas = 1

		// here we need to check partitions
		// see: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#partitions
		if statefulSet.Spec.UpdateStrategy.RollingUpdate != nil && statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			partition = int(*statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)
		}

		if statefulSet.Spec.Replicas != nil {
			replicas = int(*statefulSet.Spec.Replicas)
		}

		expectedReplicas := replicas - partition

		if int(statefulSet.Status.UpdatedReplicas) != expectedReplicas {
			return false, nil
		}

		if int(statefulSet.Status.ReadyReplicas) != replicas {
			return false, nil
		}

		// TODO delete old code
		//for _, condition := range statefulSet.Status.Conditions {
		//	if condition.Status != v1.ConditionTrue {
		//		return false, nil
		//	}
		//}

		return true, err
	case TerminatedState:
		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	default:
		return false, fmt.Errorf("state '%s' not supported", inState)
	}
}

func (pt *Tracker) podInState(inState State, object *resource) (bool, error) {
	podsClient := pt.client.CoreV1().Pods(object.namespace)
	pod, err := podsClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	switch inState {
	case ReadyState:
		if err != nil {
			return false, err
		}
		if pod.Status.Phase != v1.PodRunning {
			return false, nil
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Status != v1.ConditionTrue {
				return false, nil
			}
		}
		//deletion timestamp determines whether pod is terminating or running (nil == running)
		return pod.ObjectMeta.DeletionTimestamp == nil, nil
	case TerminatedState:
		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	default:
		return false, fmt.Errorf("state '%s' not supported", inState)
	}
}

func (pt *Tracker) daemonSetInState(inState State, object *resource) (bool, error) {
	daemonSetClient := pt.client.AppsV1().DaemonSets(object.namespace)
	daemonSet, err := daemonSetClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	switch inState {
	case ReadyState:
		if err != nil {
			return false, err
		}
		if daemonSet.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
			return true, nil
		}
		if daemonSet.Status.UpdatedNumberScheduled != daemonSet.Status.DesiredNumberScheduled {
			return false, nil
		}

		maxUnavailable, err := intstr.GetScaledValueFromIntOrPercent(daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable, int(daemonSet.Status.DesiredNumberScheduled), true)
		if err != nil {
			maxUnavailable = int(daemonSet.Status.DesiredNumberScheduled)
		}

		actualReady := int(daemonSet.Status.NumberReady)
		expectedReady := int(daemonSet.Status.DesiredNumberScheduled) - maxUnavailable
		return actualReady >= expectedReady, nil

	case TerminatedState:
		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	default:
		return false, fmt.Errorf("state '%s' not supported", inState)
	}
}

func (pt *Tracker) jobInState(inState State, object *resource) (bool, error) {
	jobClient := pt.client.BatchV1().Jobs(object.namespace)
	job, err := jobClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	switch inState {
	case ReadyState:
		if err != nil {
			return false, err
		}
		for _, condition := range job.Status.Conditions {
			if condition.Status != v1.ConditionTrue {
				return false, nil
			}
		}
		return true, err
	case TerminatedState:
		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	default:
		return false, fmt.Errorf("state '%s' not supported", inState)
	}
}
