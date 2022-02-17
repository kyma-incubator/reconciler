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

type OccupancyTracker struct {
	logger      *zap.SugaredLogger
	occupancyID string
	callbackURL string
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
				if t.callbackURL != "" && !pool.IsClosed() {
					t.logger.Info("occupancy tracker is deleting Worker Pool occupancy")
					occupancyURL, err := t.occupancyCallbackURL()
					if err != nil {
						t.logger.Error(err.Error())
						return
					}
					t.deleteWorkerPoolOccupancy(occupancyURL)
					return
				}

			case <-ticker.C:
				if t.callbackURL != "" && !pool.IsClosed() {
					occupancyURL, err := t.occupancyCallbackURL()
					if err != nil {
						t.logger.Error(err.Error())
						break
					}
					t.createOrUpdateComponentReconcilerOccupancy(occupancyURL, reconcilerName, pool.RunningWorkers(), pool.Size())
				}
			}
		}

	}()
}

func (t *OccupancyTracker) createOrUpdateComponentReconcilerOccupancy(occupancyCallbackURL, reconcilerName string, runningWorkers, poolSize int) {
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
	resp, err := http.Post(occupancyCallbackURL, "application/json", bytes.NewBuffer(jsonPayload))
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

func (t *OccupancyTracker) deleteWorkerPoolOccupancy(occupancyCallbackURL string) {
	if occupancyCallbackURL != "" {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		req, err := http.NewRequest(http.MethodDelete, occupancyCallbackURL, nil)
		if err != nil {
			t.logger.Error(err.Error())
		}
		_, err = client.Do(req)
		if err != nil {
			t.logger.Warn(err.Error())
		}
		t.logger.Infof("occupancy tracker deleted occupancy successfully for %s service", t.occupancyID)
	}
}

func (t *OccupancyTracker) occupancyCallbackURL() (string, error) {
	if t.callbackURL == "" {
		return "", fmt.Errorf("occupancy tracker failed to parse callback URL: received empty string")
	}
	u, err := url.Parse(t.callbackURL)
	if err != nil {
		return "", fmt.Errorf("occupancy tracker failed to parse callback URL: %s", err)
	}
	occupancyURLTemplate := "%s://%s:%s/v1/occupancy/%s"
	return fmt.Sprintf(occupancyURLTemplate, u.Scheme, u.Hostname(), u.Port(), t.occupancyID), nil
}

func (t *OccupancyTracker) AssignCallbackURL(callbackURL string) {
	t.callbackURL = callbackURL
}
