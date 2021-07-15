package compreconciler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httputil"

	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"net/http"
	"time"

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
	job            *scheduler.Job //trigger for http-calls to reconciler-controller
	interval       time.Duration  //interval for sending the latest status to reconciler-controller
	callbackURL    string         //URL of the reconciler-controller
	status         Status         //current status
	lastUpdate     time.Time      //time when the last status update was successfully send to reconciler-controller
	retryTimeout   time.Duration  //timeout until the status updater will stop retrying to send updates to the reconciler
	createdObjects []K8SObject
	client         *kubernetes.Clientset
}

type K8SObject struct {
	Kind      string
	Name      string
	Namespace string
}

func newStatusUpdater(interval time.Duration, callbackURL string, retryTimeout time.Duration, client *kubernetes.Clientset) *StatusUpdater {
	return &StatusUpdater{
		callbackURL:  callbackURL,
		interval:     interval,
		status:       Running,
		retryTimeout: retryTimeout,
		client:       client,
		lastUpdate:   time.Now(),
	}
}

func (su *StatusUpdater) start() error {
	task := func() {
		log, err := logger.NewLogger(true)
		if err != nil {
			log = zap.NewNop()
		}

		if su.createdObjects == nil {
			return
		}

		componentIsReady := true
		for _, object := range su.createdObjects {
			if object.Kind == "Deployment" {
				if deploymentIsReady(su, object, log) {
					componentIsReady = true
				} else {
					componentIsReady = false
					break
				}
			} else if object.Kind == "Pod" {
				if podIsReady(su, object, log) {
					componentIsReady = true
				} else {
					componentIsReady = false
					break
				}
			} else if object.Kind == "DaemonSet" {
				if daemonSetIsReady(su, object, log) {
					componentIsReady = true
				} else {
					componentIsReady = false
					break
				}
			} else if object.Kind == "StatefulSet" {
				if statefulSetIsReady(su, object, log) {
					componentIsReady = true
				} else {
					componentIsReady = false
					break
				}
			} else if object.Kind == "Job" {
				if jobIsReady(su, object, log) {
					componentIsReady = true
				} else {
					componentIsReady = false
					break
				}
			}
		}
		if componentIsReady {
			su.Success()
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

	job, err := scheduler.Every(int(5)).Seconds().Run(task)
	if err != nil {
		return err
	}
	su.job = job

	return nil
}

func deploymentIsReady(su *StatusUpdater, object K8SObject, log *zap.Logger) bool {
	deploymentsClient := su.client.AppsV1().Deployments(object.Namespace)
	deployment, err := deploymentsClient.Get(context.TODO(), object.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err.Error())
	}
	objectIsReady := true
	for _, condition := range deployment.Status.Conditions {
		objectIsReady = condition.Status == "True"
	}
	return objectIsReady
}

func statefulSetIsReady(su *StatusUpdater, object K8SObject, log *zap.Logger) bool {
	statefulSetClient := su.client.AppsV1().StatefulSets(object.Namespace)
	statefulSet, err := statefulSetClient.Get(context.TODO(), object.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err.Error())
	}
	objectIsReady := true
	for _, condition := range statefulSet.Status.Conditions {
		objectIsReady = condition.Status == "True"
	}
	return objectIsReady
}

func podIsReady(su *StatusUpdater, object K8SObject, log *zap.Logger) bool {
	podsClient := su.client.CoreV1().Pods(object.Namespace)
	pod, err := podsClient.Get(context.TODO(), object.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err.Error())
	}
	return pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodSucceeded
}

func daemonSetIsReady(su *StatusUpdater, object K8SObject, log *zap.Logger) bool {
	daemonSetClient := su.client.AppsV1().DaemonSets(object.Namespace)
	daemonSet, err := daemonSetClient.Get(context.TODO(), object.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err.Error())
	}
	objectIsReady := true
	for _, condition := range daemonSet.Status.Conditions {
		objectIsReady = condition.Status == "True"
	}
	return objectIsReady
}

func jobIsReady(su *StatusUpdater, object K8SObject, log *zap.Logger) bool {
	jobClient := su.client.BatchV1().Jobs(object.Namespace)
	job, err := jobClient.Get(context.TODO(), object.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err.Error())
	}
	objectIsReady := true
	for _, condition := range job.Status.Conditions {
		objectIsReady = condition.Status == "True"
	}
	return objectIsReady
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
