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

type occupancyTracker struct {
	logger               *zap.SugaredLogger
	occupancyID          string
	occupancyCallbackURL string
	poolSize             int
}

func newOccupancyTracker(debug bool, poolSize int) *occupancyTracker {
	return &occupancyTracker{
		logger:      logger.NewLogger(debug),
		occupancyID: getHostname(),
		poolSize:    poolSize,
	}
}

func (t *occupancyTracker) Track(ctx context.Context, pool *WorkerPool, reconcilerName string) {
	go func() {
		ticker := time.NewTicker(defaultInterval)
		for {
			select {
			case <-ctx.Done():
				if pool.CallbackURL != "" && !pool.IsClosed() {
					t.logger.Info("Deleting Worker Pool Occupancy")
					t.setOccupancyCallbackURL(pool.CallbackURL, t.occupancyID)
					t.deleteWorkerPoolOccupancy()
					return
				}

			case <-ticker.C:
				if pool.CallbackURL != "" && !pool.IsClosed() {
					t.setOccupancyCallbackURL(pool.CallbackURL, t.occupancyID)
					t.createOrUpdateComponentReconcilerOccupancy(reconcilerName, pool.RunningWorkers())
				}
			}
		}

	}()
}

func (t *occupancyTracker) createOrUpdateComponentReconcilerOccupancy(reconcilerName string, runningWorkers int) {
	httpOccupancyUpdateRequest := reconciler.HTTPOccupancyRequest{
		Component:      reconcilerName,
		RunningWorkers: runningWorkers,
		PoolSize:       t.poolSize,
	}
	jsonPayload, err := json.Marshal(httpOccupancyUpdateRequest)
	if err != nil {
		t.logger.Errorf("failed to marshal HTTP payload to update occupancy of component '%s': %s", reconcilerName, err)
		return
	}
	resp, err := http.Post(t.occupancyCallbackURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		t.logger.Error(err.Error())
		return
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		t.logger.Errorf("mothership failed to update occupancy for '%s' component with status code: '%d'", reconcilerName, resp.StatusCode)
		return
	}

	t.logger.Infof("Component reconciler '%s' updated occupancy successfully", reconcilerName)
}

func (t *occupancyTracker) deleteWorkerPoolOccupancy() {
	if t.occupancyCallbackURL != "" {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		req, err := http.NewRequest(http.MethodDelete, t.occupancyCallbackURL, nil)
		if err != nil {
			t.logger.Error(err.Error())
		}
		_, err = client.Do(req)
		if err != nil {
			t.logger.Warn(err.Error())
		}
	}
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

func (t *occupancyTracker) setOccupancyCallbackURL(callbackURL string, poolID string) {
	if callbackURL == "" {
		t.logger.Warnf("error parsing callback URL: received empty string")
		return
	}
	u, err := url.Parse(callbackURL)
	if err != nil {
		t.logger.Warnf("error parsing callback URL: %s", err)
		return
	}
	occupancyURLTemplate := "%s://%s:%s/v1/occupancy/%s"
	t.occupancyCallbackURL = fmt.Sprintf(occupancyURLTemplate, u.Scheme, u.Hostname(), u.Port(), poolID)
}
