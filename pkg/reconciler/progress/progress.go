package progress

import (
	"context"
	"fmt"
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
)

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
	ctx      context.Context
	objects  []*resource
	client   *kubernetes.Clientset
	interval time.Duration
	timeout  time.Duration
	logger   *zap.SugaredLogger
}

func NewProgressTracker(ctx context.Context, client *kubernetes.Clientset, logger *zap.SugaredLogger, config Config) (*Tracker, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &Tracker{
		ctx:      ctx,
		client:   client,
		interval: config.Interval,
		timeout:  config.Timeout,
		logger:   logger,
	}, nil
}

func (pt *Tracker) Watch() error {
	if len(pt.objects) == 0 { //check if any watchable resources were added
		pt.logger.Debug("No watchable resources defined: installation treated as successfully finished")
		return nil
	}

	//initial installation status check
	ready, err := pt.isReady()
	if err != nil {
		pt.logger.Warnf("Failed to verify initial Kubernetes resource installation status: %v", err)
	}
	if ready {
		//we are already done
		pt.logger.Debug("Watchable resources are already installed")
		return nil
	}

	//start verifying the installation status in an interval
	readyCheck := time.NewTicker(pt.interval)
	timeout := time.After(pt.timeout)
	for {
		select {
		case <-readyCheck.C:
			ready, err := pt.isReady()
			if err != nil {
				pt.logger.Warnf("Failed to check Kubernetes resource installation progress but will "+
					"retry until timeout is reached: %s", err)
			}
			if ready {
				readyCheck.Stop()
				return nil
			}
		case <-pt.ctx.Done():
			pt.logger.Debug("Stopping progress tracker because parent context got closed")
			return &e.ContextClosedError{
				Message: "Progress tracker interrupted: running installation treated as non-successfully installed",
			}
		case <-timeout:
			err := fmt.Errorf("progress tracker reached timeout (%.0f secs): stop checking resource installation state",
				pt.timeout.Seconds())
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

func (pt *Tracker) isReady() (bool, error) {
	var err error
	componentIsReady := true
	for _, object := range pt.objects {
		switch object.kind {
		case Pod:
			componentIsReady, err = pt.podIsReady(object)
		case Deployment:
			componentIsReady, err = pt.deploymentIsReady(object)
		case DaemonSet:
			componentIsReady, err = pt.daemonSetIsReady(object)
		case StatefulSet:
			componentIsReady, err = pt.statefulSetIsReady(object)
		case Job:
			componentIsReady, err = pt.jobIsReady(object)
		}
		pt.logger.Debugf("%s resource '%s:%s' is ready: %t", object.kind, object.name, object.namespace, componentIsReady)
		if err != nil {
			pt.logger.Errorf("Failed to retrieve installation progress of %v: %s", object, err)
			return false, err
		}
		if !componentIsReady { //at least one component is not ready
			pt.logger.Debug("Installation is still ongoing")
			return false, nil
		}
	}
	pt.logger.Debug("Installation finished successfully")
	return componentIsReady, nil
}

func (pt *Tracker) deploymentIsReady(object *resource) (bool, error) {
	deploymentsClient := pt.client.AppsV1().Deployments(object.namespace)
	deployment, err := deploymentsClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, condition := range deployment.Status.Conditions {
		if condition.Status != v1.ConditionTrue {
			return false, nil
		}
	}
	return true, err
}

func (pt *Tracker) statefulSetIsReady(object *resource) (bool, error) {
	statefulSetClient := pt.client.AppsV1().StatefulSets(object.namespace)
	statefulSet, err := statefulSetClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, condition := range statefulSet.Status.Conditions {
		if condition.Status != v1.ConditionTrue {
			return false, nil
		}
	}
	return true, err
}

func (pt *Tracker) podIsReady(object *resource) (bool, error) {
	podsClient := pt.client.CoreV1().Pods(object.namespace)
	pod, err := podsClient.Get(context.TODO(), object.name, metav1.GetOptions{})
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
}

func (pt *Tracker) daemonSetIsReady(object *resource) (bool, error) {
	daemonSetClient := pt.client.AppsV1().DaemonSets(object.namespace)
	daemonSet, err := daemonSetClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, condition := range daemonSet.Status.Conditions {
		if condition.Status != v1.ConditionTrue {
			return false, nil
		}
	}
	return true, err
}

func (pt *Tracker) jobIsReady(object *resource) (bool, error) {
	jobClient := pt.client.BatchV1().Jobs(object.namespace)
	job, err := jobClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, condition := range job.Status.Conditions {
		if condition.Status != v1.ConditionTrue {
			return false, nil
		}
	}
	return true, err
}
