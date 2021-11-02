package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"

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

func StartWebserver(ctx context.Context, o *reconCli.Options, workerPool *service.WorkerPool) error {
	srv := server.Webserver{
		Logger:     o.Logger(),
		Port:       o.ServerConfig.Port,
		SSLCrtFile: o.ServerConfig.SSLCrtFile,
		SSLKeyFile: o.ServerConfig.SSLKeyFile,
		Router:     newRouter(ctx, o, workerPool),
	}
	return srv.Start(ctx) //blocking until ctx gets closed
}

func newRouter(ctx context.Context, o *reconCli.Options, workerPool *service.WorkerPool) *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/run", paramContractVersion),
		func(w http.ResponseWriter, r *http.Request) { //just an adapter for the reconcile-fct call
			reconcile(ctx, w, r, o, workerPool)
		},
	).Methods("PUT", "POST")
	return router
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

func reconcile(ctx context.Context, w http.ResponseWriter, req *http.Request, o *reconCli.Options, workerPool *service.WorkerPool) {
	dump, err := httputil.DumpRequest(req, true)
	if err == nil {
		o.Logger().Debug("Start processing reconciliation request: %s", string(dump))
	} else {
		o.Logger().Warnf("REST endpoint failed to dump http request for debugging purposes: %s", err)
	}

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

	//check whether all dependencies are fulfilled
	depCheck := workerPool.CheckDependencies(model)
	if depCheck.DependencyMissing() {
		o.Logger().Debugf("Model '%s' not reconcilable because dependencies are missing: '%s'",
			model, strings.Join(depCheck.Missing, "', '"))
		server.SendHTTPError(w, http.StatusPreconditionRequired, reconciler.HTTPMissingDependenciesResponse{
			Dependencies: reconciler.Dependencies{
				Required: depCheck.Required,
				Missing:  depCheck.Missing,
			},
		})
		return
	}

	o.Logger().Debugf("Assigning reconciliation worker to model '%s'", model)
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
