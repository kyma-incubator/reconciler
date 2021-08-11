package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func startWebserver(ctx context.Context, o *Options) error {
	//routing
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters", paramContractVersion),
		callHandler(o, createOrUpdateCluster)).
		Methods("PUT", "POST")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}", paramContractVersion, paramCluster),
		callHandler(o, deleteCluster)).
		Methods("DELETE")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/configs/{%s}/status", paramContractVersion, paramCluster, paramConfigVersion),
		callHandler(o, getCluster)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/status", paramContractVersion, paramCluster),
		callHandler(o, getLatestCluster)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/statusChanges/{%s}", paramContractVersion, paramCluster, paramOffset),
		callHandler(o, statusChanges)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/operations/{%s}/callback/{%s}", paramContractVersion, paramSchedulingID, paramCorrelationID),
		callHandler(o, operationCallback)).
		Methods("POST")

	//metrics endpoint
	metrics.RegisterAll(o.Registry.Inventory(), o.Logger())
	router.Handle("/metrics", promhttp.Handler())

	//start server process
	srv := &server.Webserver{
		Logger:     o.Logger(),
		Port:       o.Port,
		SSLCrtFile: o.SSLCrt,
		SSLKeyFile: o.SSLKey,
		Router:     router,
	}
	return srv.Start(ctx) //blocking call
}

func callHandler(o *Options, handler func(o *Options, w http.ResponseWriter, r *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(o, w, r)
	}
}

func createOrUpdateCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	contractV, err := params.Int64(paramContractVersion)
	if err != nil {
		sendError(w, http.StatusBadRequest, errors.Wrap(err, "Contract version undefined"))
		return
	}
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to read received JSON payload"))
		return
	}
	clusterModel, err := keb.NewModelFactory(contractV).Cluster(reqBody)
	if err != nil {
		sendError(w, http.StatusBadRequest, errors.Wrap(err, "Failed to unmarshal JSON payload"))
		return
	}
	clusterState, err := o.Registry.Inventory().CreateOrUpdate(contractV, clusterModel)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to create or update cluster entity"))
		return
	}
	//respond status URL
	payload := responsePayload(clusterState)
	payload["statusUrl"] = fmt.Sprintf("%s%s/%s/configs/%d/status", r.Host, r.URL.RequestURI(), clusterState.Cluster.Cluster, clusterState.Configuration.Version)
	sendResponse(w, payload)
}

func getCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	clusterName, err := params.String(paramCluster)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	configVersion, err := params.Int64(paramConfigVersion)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	clusterState, err := o.Registry.Inventory().Get(clusterName, configVersion)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Could not retrieve cluster state"))
		return
	}
	sendResponse(w, responsePayload(clusterState))
}

func getLatestCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	clusterName, err := params.String(paramCluster)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	clusterState, err := o.Registry.Inventory().GetLatest(clusterName)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Could not retrieve cluster state"))
		return
	}
	sendResponse(w, responsePayload(clusterState))
}

func statusChanges(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	clusterName, err := params.String(paramCluster)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	offset, err := params.String(paramOffset)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	duration, err := time.ParseDuration(offset)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	changes, err := o.Registry.Inventory().StatusChanges(clusterName, duration)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Could not retrieve cluster statusChanges"))
		return
	}
	//respond
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(changes); err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to encode cluster statusChanges response"))
		return
	}
}

func deleteCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	clusterName, err := params.String(paramCluster)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := o.Registry.Inventory().GetLatest(clusterName); repository.IsNotFoundError(err) {
		sendError(w, http.StatusNotFound, errors.Wrap(err, fmt.Sprintf("Deletion impossible: Cluster '%s' not found", clusterName)))
		return
	}
	if err := o.Registry.Inventory().Delete(clusterName); err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, fmt.Sprintf("Failed to delete cluster '%s'", clusterName)))
		return
	}
}

func operationCallback(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	schedulingID, err := params.String(paramSchedulingID)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	correlationID, err := params.String(paramCorrelationID)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}

	var body reconciler.CallbackMessage
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to read received JSON payload"))
		return
	}
	err = json.Unmarshal(reqBody, &body)
	if err != nil {
		sendError(w, http.StatusBadRequest, errors.Wrap(err, "Failed to unmarshal JSON payload"))
		return
	}
	switch body.Status {
	case string(reconciler.NotStarted), string(reconciler.Running):
		err = o.Registry.OperationsRegistry().SetInProgress(correlationID, schedulingID)
	case string(reconciler.Success):
		err = o.Registry.OperationsRegistry().SetDone(correlationID, schedulingID)
	case string(reconciler.Error):
		err = o.Registry.OperationsRegistry().SetError(correlationID, schedulingID, "Reconciler reported error status")
	case string(reconciler.Failed):
		err = o.Registry.OperationsRegistry().SetFailed(correlationID, schedulingID, "Reconciler reported failed status")
	}
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
}

func responsePayload(clusterState *cluster.State) map[string]interface{} {
	return map[string]interface{}{
		"cluster":              clusterState.Cluster.Cluster,
		"clusterVersion":       clusterState.Cluster.Version,
		"configurationVersion": clusterState.Configuration.Version,
		"status":               clusterState.Status.Status,
	}
}

func sendError(w http.ResponseWriter, httpCode int, err error) {
	http.Error(w, fmt.Sprintf("%s\n\n%s", http.StatusText(httpCode), err.Error()), httpCode)
}

func sendResponse(w http.ResponseWriter, payload map[string]interface{}) {
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to encode response payload to JSON"))
	}
}
