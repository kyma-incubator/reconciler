package occupancy

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/worker"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"strings"
	"time"
)

const (
	defaultOccupancyCleanUpInterval  = 5 * time.Minute
	defaultOccupancyTrackingInterval = 30 * time.Second
	mothershipScalableServiceName    = "mothership"
	defaultKcpNamespace              = "kcp-system"
	nameLabelSelector                = "app.kubernetes.io/name:"
)

type Tracker struct {
	occupancyID          string
	workerPool           *worker.Pool
	poolSize             int
	repo                 Repository
	logger               *zap.SugaredLogger
	scalableServiceNames []string
}

func NewTracker(workerPool *worker.Pool, repo Repository, cfg *config.Config, poolSize int, debug bool) *Tracker {
	return &Tracker{
		//using hostname (= pod name) as the id to be able
		//to clean up pods that have died w/o being able to delete their occupancy
		occupancyID:          getHostname(),
		workerPool:           workerPool,
		poolSize:             poolSize,
		repo:                 repo,
		logger:               logger.NewLogger(debug),
		scalableServiceNames: getReconcilers(cfg),
	}
}

func (t *Tracker) Run(ctx context.Context) {

	//create in-cluster K8s client
	clientset, err := createK8sInClusterClientSet()
	if err != nil {
		return
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
			_, err = t.repo.CreateOrUpdateWorkerPoolOccupancy(t.occupancyID, mothershipScalableServiceName, runningWorkers, t.poolSize)
			if err != nil {
				t.logger.Errorf("could not create/update occupancy for %s: %s", t.occupancyID, err)
			}
		case <-cleanupTicker.C:
			err = t.cleanUpOrphanOccupancies(clientset)
			if err != nil {
				t.logger.Errorf("could not cleanup orphan occupancies : %s", err)
				break
			}
			<-ctx.Done()
			if !t.workerPool.IsClosed() {
				t.logger.Info("Deleting Worker Pool Occupancy")
				trackingTicker.Stop()
				cleanupTicker.Stop()
				err := t.repo.RemoveWorkerPoolOccupancy(t.occupancyID)
				if err != nil {
					t.logger.Errorf("could not delete occupancy for %s: %s", t.occupancyID, err)
				}
				return
			}
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

func (t *Tracker) getScalablePodNames(clientset *kubernetes.Clientset) ([]string, error) {
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

func (t *Tracker) cleanUpOrphanOccupancies(clientset *kubernetes.Clientset) error {
	scalablePodNames, err := t.getScalablePodNames(clientset)
	if err != nil {
		return err
	}
	//TODO: implement get occupancy IDs
	componentsIDs, err := t.repo.GetComponentList()
	if err != nil {
		return err
	}
	for _, componentId := range componentsIDs {
		found := binarySearch(componentId, scalablePodNames)
		if !found {
			err = t.repo.RemoveWorkerPoolOccupancy(componentId)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func binarySearch(name string, components []string) bool {
	//TODO: implement
	return false
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

func getReconcilers(cfg *config.Config) []string {
	reconcilerList := make([]string, 0, len(cfg.Scheduler.Reconcilers)+1)
	for reconciler := range cfg.Scheduler.Reconcilers {
		formattedReconciler := strings.Replace(reconciler, "-", "_", -1)
		reconcilerList = append(reconcilerList, formattedReconciler)
	}
	return append(reconcilerList, "mothership")
}
