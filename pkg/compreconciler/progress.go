package compreconciler

import (
	"context"
	"fmt"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

const (
	defaultInterval = 20 * time.Second
	defaultTimeout  = 10 * time.Minute
)

type k8sObject struct {
	kind      WatchableResource
	name      string
	namespace string
}

func (o *k8sObject) String() string {
	return fmt.Sprintf("%s [namespace:%s|name:%s]", o.kind, o.namespace, o.name)
}

type ProgressTrackerConfig struct {
	interval time.Duration
	timeout  time.Duration
}

func (ptc *ProgressTrackerConfig) validate() error {
	if ptc.interval < 0 {
		return fmt.Errorf("progress tracker status-check interval cannot be < 0")
	}
	if ptc.interval == 0 {
		ptc.interval = defaultInterval
	}
	if ptc.timeout < 0 {
		return fmt.Errorf("progress tracker timeout cannot be < 0")
	}
	if ptc.timeout == 0 {
		ptc.timeout = defaultTimeout
	}
	if ptc.timeout <= ptc.interval {
		return fmt.Errorf("progress tracker will never run because configured timeout "+
			"is <= as the check interval :%.0f secs <= %.0f secs", ptc.timeout.Seconds(), ptc.interval.Seconds())
	}
	return nil
}

type ProgressTracker struct {
	objects  []*k8sObject
	client   *kubernetes.Clientset
	interval time.Duration
	timeout  time.Duration
	logger   *zap.Logger
}

func NewProgressTracker(client *kubernetes.Clientset, debug bool, config ProgressTrackerConfig) (*ProgressTracker, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	logger, err := log.NewLogger(debug)
	if err != nil {
		return nil, err
	}

	return &ProgressTracker{
		client:   client,
		interval: config.interval,
		timeout:  config.timeout,
		logger:   logger,
	}, nil
}

func (pt *ProgressTracker) Watch() error {
	if len(pt.objects) == 0 { //check if any watchable resources were added
		pt.logger.Debug("No watchable resources defined: installation treated as successfully finished")
		return nil
	}

	//initial installation status check
	ready, err := pt.isReady()
	if err == nil {
		pt.logger.Warn(fmt.Sprintf("Failed to run initial Kubernetes resource installation status: %s", err))
		if ready {
			//we are already done
			return nil
		}
	}

	//start verifying the installation status in an interval
	readyCheck := time.NewTicker(pt.interval)
	timeout := time.After(pt.timeout)
	for {
		select {
		case <-readyCheck.C:
			ready, err := pt.isReady()
			if err != nil {
				pt.logger.Warn(fmt.Sprintf("Failed to check Kubernetes resource installation progress but will "+
					"retry until timeout is reached: %s", err))
			}
			if ready {
				readyCheck.Stop()
				return nil
			}
		case <-timeout:
			err := fmt.Errorf("progress tracker reached timeout (%.0f secs): stop checking resource installation state",
				pt.timeout.Seconds())
			pt.logger.Warn(err.Error())
			return err
		}
	}
}

func (pt *ProgressTracker) AddResource(kind WatchableResource, namespace, name string) {
	pt.objects = append(pt.objects, &k8sObject{
		kind:      kind,
		namespace: namespace,
		name:      name,
	})
}

func (pt *ProgressTracker) isReady() (bool, error) {
	var err error
	componentIsReady := true
	for _, object := range pt.objects {
		switch object.kind {
		case Deployment:
			componentIsReady, err = pt.deploymentIsReady(object)
		case Pod:
			componentIsReady, err = pt.podIsReady(object)
		case DaemonSet:
			componentIsReady, err = pt.daemonSetIsReady(object)
		case StatefulSet:
			componentIsReady, err = pt.statefulSetIsReady(object)
		case Job:
			componentIsReady, err = pt.jobIsReady(object)
		}
		pt.logger.Debug(fmt.Sprintf("%s resource '%s:%s' is ready: %t", object.kind, object.name, object.namespace, componentIsReady))
		if err != nil {
			pt.logger.Error(fmt.Sprintf("Failed to retrieve installation progress of %v: %s", object, err))
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

func (pt *ProgressTracker) deploymentIsReady(object *k8sObject) (bool, error) {
	deploymentsClient := pt.client.AppsV1().Deployments(object.namespace)
	deployment, err := deploymentsClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	objectIsReady := true
	for _, condition := range deployment.Status.Conditions {
		objectIsReady = condition.Status == "True"
	}
	return objectIsReady, nil
}

func (pt *ProgressTracker) statefulSetIsReady(object *k8sObject) (bool, error) {
	statefulSetClient := pt.client.AppsV1().StatefulSets(object.namespace)
	statefulSet, err := statefulSetClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	objectIsReady := true
	for _, condition := range statefulSet.Status.Conditions {
		objectIsReady = condition.Status == "True"
	}
	return objectIsReady, nil
}

func (pt *ProgressTracker) podIsReady(object *k8sObject) (bool, error) {
	podsClient := pt.client.CoreV1().Pods(object.namespace)
	pod, err := podsClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	objectIsReady := pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodRunning
	return objectIsReady, nil
}

func (pt *ProgressTracker) daemonSetIsReady(object *k8sObject) (bool, error) {
	daemonSetClient := pt.client.AppsV1().DaemonSets(object.namespace)
	daemonSet, err := daemonSetClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	objectIsReady := true
	for _, condition := range daemonSet.Status.Conditions {
		objectIsReady = condition.Status == "True"
	}
	return objectIsReady, nil
}

func (pt *ProgressTracker) jobIsReady(object *k8sObject) (bool, error) {
	jobClient := pt.client.BatchV1().Jobs(object.namespace)
	job, err := jobClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	objectIsReady := true
	for _, condition := range job.Status.Conditions {
		objectIsReady = condition.Status == "True"
	}
	return objectIsReady, nil
}
