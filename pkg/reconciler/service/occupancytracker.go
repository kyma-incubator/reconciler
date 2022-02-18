package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"os"
	"time"
)

const occupancyURLTemplate = "%s://%s:%s/v1/occupancy/%s"

type OccupancyTracker struct {
	logger               *zap.SugaredLogger
	occupancyID          string
	occupancyCallbackURL string
}

func newOccupancyTracker(debug bool) *OccupancyTracker {
	return &OccupancyTracker{
		logger: logger.NewLogger(debug),
	}
}

func (t *OccupancyTracker) Track(ctx context.Context, pool *WorkerPool, reconcilerName string) {
	podName, err := os.Hostname()
	if err != nil {
		t.logger.Errorf("occupancy tracker could not retrieve pod name: %s", err)
		return
	}
	//using hostname (= pod name) as the id to be able
	//to clean up pods that have crashed w/o being able to delete their occupancy
	t.occupancyID = podName
	go func() {
		ticker := time.NewTicker(defaultInterval)
		for {
			select {
			case <-ctx.Done():
				if t.occupancyCallbackURL != "" && !pool.IsClosed() {
					t.logger.Info("occupancy tracker is deleting Worker Pool occupancy")
					t.deleteWorkerPoolOccupancy()
					return
				}

			case <-ticker.C:
				if t.occupancyCallbackURL != "" && !pool.IsClosed() {
					t.createOrUpdateComponentReconcilerOccupancy(reconcilerName, pool.RunningWorkers(), pool.Size())
				}
			}
		}

	}()
}

func (t *OccupancyTracker) createOrUpdateComponentReconcilerOccupancy(reconcilerName string, runningWorkers, poolSize int) {
	httpOccupancyUpdateRequest := reconciler.HTTPOccupancyRequest{
		Component:      reconcilerName,
		RunningWorkers: runningWorkers,
		PoolSize:       poolSize,
	}
	jsonPayload, err := json.Marshal(httpOccupancyUpdateRequest)
	if err != nil {
		t.logger.Errorf("occupancy tracker failed to marshal HTTP payload to update occupancy of service '%s': %s", t.occupancyID, err)
		return
	}
	resp, err := http.Post(t.occupancyCallbackURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		t.logger.Error(err.Error())
		return
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		t.logger.Errorf("mothership failed to update occupancy for '%s' service with status code: '%d'", t.occupancyID, resp.StatusCode)
		return
	}

	t.logger.Infof("occupancy tracker updated occupancy successfully for %s service", t.occupancyID)
}

func (t *OccupancyTracker) deleteWorkerPoolOccupancy() {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest(http.MethodDelete, t.occupancyCallbackURL, nil)
	if err != nil {
		t.logger.Error(err.Error())
		return
	}
	_, err = client.Do(req)
	if err != nil {
		t.logger.Error(err.Error())
		return
	}
	t.logger.Infof("occupancy tracker deleted occupancy successfully for %s service", t.occupancyID)
}

func parseOccupancyCallbackURL(callbackURL, occupancyID string) (string, error) {
	if callbackURL == "" {
		return "", fmt.Errorf("occupancy tracker failed to parse callback URL: received empty string")
	}
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", fmt.Errorf("occupancy tracker failed to parse callback URL: %s", err)
	}
	return fmt.Sprintf(occupancyURLTemplate, u.Scheme, u.Hostname(), u.Port(), occupancyID), nil
}

func (t *OccupancyTracker) AssignCallbackURL(callbackURL string) {
	if t.occupancyID != "" {
		var err error
		t.occupancyCallbackURL, err = parseOccupancyCallbackURL(callbackURL, t.occupancyID)
		if err != nil {
			t.logger.Errorf("occupancy tracker failed to assign callback URL: %s", err)
			return
		}
		t.logger.Debugf("occupancy tracker assigned callback URL successfully")
	}

}
