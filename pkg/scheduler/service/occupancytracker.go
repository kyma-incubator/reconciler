package service

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/worker"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"time"
)

const (
	defaultOccupancyCleanUpInterval  = 5 * time.Minute
	defaultOccupancyTrackingInterval = 30 * time.Second
	mothershipScalableServiceName    = "mothership"
	defaultKcpNamespace              = "kcp-system"
	nameLabelSelector                = "app.kubernetes.io/name:"
)

type OccupancyTracker struct {
	occupancyID          string
	workerPool           *worker.Pool
	repo                 occupancy.Repository
	logger               *zap.SugaredLogger
	scalableServiceNames []string
}

func NewOccupancyTracker(workerPool *worker.Pool, repo occupancy.Repository, cfg *config.Config, logger *zap.SugaredLogger) *OccupancyTracker {
	return &OccupancyTracker{
		workerPool:           workerPool,
		repo:                 repo,
		logger:               logger,
		scalableServiceNames: metrics.GetReconcilerList(cfg),
	}
}

func (t *OccupancyTracker) Run(ctx context.Context) error {
	//using hostname (= pod name) as the id to be able
	//to clean up pods that have died w/o being able to delete their occupancy
	var err error
	t.occupancyID, err = os.Hostname()
	if err != nil {
		return fmt.Errorf("occupancy tracker failed to get host name: %s", err)
	}
	//create in-cluster K8s client
	clientset, err := createK8sInClusterClientSet()
	if err != nil {
		return fmt.Errorf("occupancy tracker failed to create in-cluster clientset: %s", err)
	}
	//start occupancy tracking && cleaning
	trackingTicker := time.NewTicker(defaultOccupancyTrackingInterval)
	cleanupTicker := time.NewTicker(defaultOccupancyCleanUpInterval)
	for {
		select {
		case <-trackingTicker.C:
			runningWorkers, err := t.workerPool.RunningWorkers()
			if err != nil {
				t.logger.Errorf("could not create/update occupancy for %s: %s", t.occupancyID, err)
				break
			}
			poolSize, err := t.workerPool.Size()
			if err != nil {
				t.logger.Errorf("could not create/update occupancy for %s: %s", t.occupancyID, err)
				break
			}
			_, err = t.repo.CreateOrUpdateWorkerPoolOccupancy(t.occupancyID, mothershipScalableServiceName, runningWorkers, poolSize)
			if err != nil {
				t.logger.Errorf("could not create/update occupancy for %s: %s", t.occupancyID, err)
			}
		case <-cleanupTicker.C:
			err = t.cleanUpOrphanOccupancies(clientset)
			if err != nil {
				t.logger.Errorf("could not cleanup orphan occupancies : %s", err)
				break
			}
		case <-ctx.Done():
			t.logger.Info("Deleting Worker Pool Occupancy")
			trackingTicker.Stop()
			cleanupTicker.Stop()
			err = t.repo.RemoveWorkerPoolOccupancy(t.occupancyID)
			if err != nil {
				return err
			}
			return nil
		}
	}
}

func createK8sInClusterClientSet() (*kubernetes.Clientset, error) {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(inClusterConfig)
}

func (t *OccupancyTracker) getScalablePodNames(clientset *kubernetes.Clientset) ([]string, error) {

	var scalablePodNames []string
	for _, scalableServiceName := range t.scalableServiceNames {
		pods, err := clientset.CoreV1().Pods(defaultKcpNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s%s", nameLabelSelector, scalableServiceName),
		})
		if err != nil {
			return nil, err
		}
		for _, pod := range pods.Items {
			podName := pod.Name
			scalablePodNames = append(scalablePodNames, podName)
		}
	}

	return scalablePodNames, nil
}

func (t *OccupancyTracker) cleanUpOrphanOccupancies(clientset *kubernetes.Clientset) error {
	scalablePodNames, err := t.getScalablePodNames(clientset)
	if err != nil {
		return err
	}

	componentsIDs, err := t.repo.GetComponentIDs()
	if err != nil {
		return err
	}
	if len(componentsIDs) == 0 {
		t.logger.Warnf("occupancy tracker received empty list of ids: nothing to clean")
		return nil
	}
	for _, componentID := range componentsIDs {
		found := false
		for _, scalablePodName := range scalablePodNames {
			if componentID == scalablePodName {
				found = true
				break
			}
		}
		if !found {
			err = t.repo.RemoveWorkerPoolOccupancy(componentID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
