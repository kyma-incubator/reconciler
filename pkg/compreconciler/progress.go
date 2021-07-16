package compreconciler

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type k8sObject struct {
	kind      string
	name      string
	namespace string
}

func (o *k8sObject) String() string {
	return fmt.Sprintf("%s [namespace:%s|name:%s]", o.kind, o.namespace, o.name)
}

type progressTracker struct {
	objects []*k8sObject
	client  *kubernetes.Clientset
	logger  *zap.Logger
}

func newProgressTracker(objects []*k8sObject, client *kubernetes.Clientset, debug bool) *progressTracker {
	return &progressTracker{
		objects: objects,
		client:  client,
		logger:  newLogger(debug),
	}
}

func (pt *progressTracker) isReady() bool {
	var err error
	componentIsReady := true
	for _, object := range pt.objects {
		switch object.kind {
		case "Deployment":
			componentIsReady, err = pt.deploymentIsReady(object)
		case "Pod":
			componentIsReady, err = pt.podIsReady(object)
		case "DaemonSet":
			componentIsReady, err = pt.daemonSetIsReady(object)
		case "StatefulSet":
			componentIsReady, err = pt.statefulSetIsReady(object)
		case "Job":
			componentIsReady, err = pt.jobIsReady(object)
		}
		if err != nil {
			pt.logger.Error(fmt.Sprintf("Failed to measure progress of %v: %s", object, err))
		}
		if !componentIsReady { //at least one component is not ready
			return false
		}
	}
	return componentIsReady
}

func (pt *progressTracker) deploymentIsReady(object *k8sObject) (bool, error) {
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

func (pt *progressTracker) statefulSetIsReady(object *k8sObject) (bool, error) {
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

func (pt *progressTracker) podIsReady(object *k8sObject) (bool, error) {
	podsClient := pt.client.CoreV1().Pods(object.namespace)
	pod, err := podsClient.Get(context.TODO(), object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	objectIsReady := pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodRunning
	return objectIsReady, nil
}

func (pt *progressTracker) daemonSetIsReady(object *k8sObject) (bool, error) {
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

func (pt *progressTracker) jobIsReady(object *k8sObject) (bool, error) {
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
