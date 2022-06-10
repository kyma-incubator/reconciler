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
	"sync"
	"time"
)

const (
	occupancyURLTemplate   = "%s://%s:%s/v1/occupancy/%s"
	defaultBackOffInterval = 2 * time.Minute
)

type OccupancyTracker struct {
	logger               *zap.SugaredLogger
	occupancyID          string
	occupancyCallbackURL string
	ticker               *time.Ticker
	sync.Mutex
}

func newOccupancyTracker(debug bool) *OccupancyTracker {
	return &OccupancyTracker{
		logger: logger.NewLogger(debug),
		ticker: time.NewTicker(defaultInterval),
	}
}

func (t *OccupancyTracker) Track(ctx context.Context, pool *WorkerPool, reconcilerName string) {
	podName, err := os.Hostname()
	if err != nil {
		t.logger.Errorf("occupancy tracker is failing: could not retrieve pod name: %s", err)
		return
	}
	//using hostname (= pod name) as the id to be able
	//to clean up pods that have crashed w/o being able to delete their occupancy
	t.occupancyID = podName
	go func() {
		for {
			select {

			case <-t.ticker.C:
				if t.occupancyCallbackURL != "" && !pool.IsClosed() {
					t.createOrUpdateComponentReconcilerOccupancy(reconcilerName, pool.RunningWorkers(), pool.Size())
				}

			case <-ctx.Done():
				if t.occupancyCallbackURL != "" {
					t.logger.Info("occupancy tracker is stopping and deleting worker pool occupancy")
					t.ticker.Stop()
					t.deleteWorkerPoolOccupancy()
					return
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
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			t.logger.Infof("occupancy tracker failed to close HTTP response body: %s", err)
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		if resp.StatusCode == http.StatusNotFound {
			t.logger.Debugf("occupancy tracker is setting update interval to its back off value: %v", defaultBackOffInterval)
			t.ticker.Reset(defaultBackOffInterval)
		}
		t.logger.Warnf("occupancy tracker received error (status code: '%d') from mothership when updating occupancy for '%s' service", resp.StatusCode, t.occupancyID)
		return
	}
	trackerAction := "updated"
	if resp.StatusCode == http.StatusCreated {
		trackerAction = "created"
	}
	t.logger.Debugf("occupancy tracker %s occupancy successfully for %s service", trackerAction, t.occupancyID)
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
	t.logger.Debugf("occupancy tracker deleted occupancy successfully for %s service", t.occupancyID)
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
	if t.occupancyCallbackURL != "" && t.ticker != nil {
		t.logger.Debugf("occupancy tracker is resetting update interval to its original value: %d", defaultInterval)
		t.ticker.Reset(defaultInterval)
	} else if t.occupancyID != "" {
		var err error
		t.Lock()
		t.occupancyCallbackURL, err = parseOccupancyCallbackURL(callbackURL, t.occupancyID)
		t.Unlock()
		if err != nil {
			t.logger.Errorf("occupancy tracker failed to assign callback URL: %s", err)
			return
		}
		t.logger.Debugf("occupancy tracker assigned callback URL successfully: %s", t.occupancyCallbackURL)
	}

}
