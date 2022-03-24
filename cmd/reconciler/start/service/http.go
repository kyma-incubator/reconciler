package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	reconCli "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"github.com/pkg/errors"
)

const (
	paramContractVersion = "version"
)

func StartWebserver(ctx context.Context, o *reconCli.Options, workerPool *service.WorkerPool, tracker *service.OccupancyTracker) error {
	srv := server.Webserver{
		Logger:     o.Logger(),
		Port:       o.ServerConfig.Port,
		SSLCrtFile: o.ServerConfig.SSLCrtFile,
		SSLKeyFile: o.ServerConfig.SSLKeyFile,
		Router:     newRouter(ctx, o, workerPool, tracker),
	}
	return srv.Start(ctx) //blocking until ctx gets closed
}

func newRouter(ctx context.Context, o *reconCli.Options, workerPool *service.WorkerPool, tracker *service.OccupancyTracker) *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/run", paramContractVersion),
		func(w http.ResponseWriter, r *http.Request) { //just an adapter for the reconcile-fct call
			reconcile(ctx, w, r, o, workerPool, tracker)
		},
	).Methods("PUT", "POST")
	metricsRouter := router.Path("/metrics").Subrouter()
	metricsRouter.Handle("", promhttp.Handler())

	//liveness and readiness checks
	router.HandleFunc("/health/live", live)
	router.HandleFunc("/health/ready", ready(workerPool))

	return router
}

func live(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func ready(workerPool *service.WorkerPool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if workerPool.IsClosed() {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func newModel(req *http.Request) (*reconciler.Task, error) {
	params := server.NewParams(req)
	contractVersion, err := params.String(paramContractVersion)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	model, err := modelForVersion(contractVersion)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, model)
	if err != nil {
		return nil, err
	}

	if model.Configuration == nil {
		model.Configuration = map[string]interface{}{}
	}

	return model, err
}

func modelForVersion(contractVersion string) (*reconciler.Task, error) {
	if contractVersion == "" {
		return nil, fmt.Errorf("contract version cannot be empty")
	}
	return &reconciler.Task{}, nil //change this function if multiple contract versions have to be supported
}

var reconcileSubmissionMutex = sync.Mutex{}

func reconcile(ctx context.Context, w http.ResponseWriter, req *http.Request, o *reconCli.Options, workerPool *service.WorkerPool, tracker *service.OccupancyTracker) {
	o.Logger().Debug("Start processing reconciliation request")

	//marshal model
	model, err := newModel(req)
	if err != nil {
		o.Logger().Warnf("Unmarshalling of model failed: %s", err)
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	o.Logger().Debugf("Reconciliation model unmarshalled: %s", model)

	//validate model
	if err := model.Validate(); err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}

	o.Logger().Debugf("Assigning reconciliation worker to model '%s'", model)
	//setting callback URL for occupancy tracking

	tracker.AssignCallbackURL(model.CallbackURL)

	// this mutex is necessary because if we have heavy parallel submissions, it can happen that the worker pool was not
	// full during the if statement execution, but got filled by another goroutine from the router and then leads to
	// ErrPoolOverload. This can only be circumvented by a small read lock in the worker-pool submission for now.
	reconcileSubmissionMutex.Lock()
	defer reconcileSubmissionMutex.Unlock()

	if workerPool.IsFull() {
		server.SendHTTPError(w, http.StatusTooManyRequests, &reconciler.HTTPErrorResponse{
			Error: errors.Errorf("worker pool for %s has reached it's capacity %v", model.Component, workerPool.Size()).Error(),
		})
		return
	}

	if err := workerPool.AssignWorker(ctx, model); err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	sendResponse(w)
}

func sendResponse(w http.ResponseWriter) {
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(&reconciler.HTTPReconciliationResponse{}); err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to encode response payload to JSON").Error(),
		})
	}
}
