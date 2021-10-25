package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/kyma-incubator/reconciler/pkg/server"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

const (
	paramContractVersion = "contractVersion"
	paramRuntimeID       = "runtimeID"
	paramConfigVersion   = "configVersion"
	paramOffset          = "offset"
	paramSchedulingID    = "schedulingID"
	paramCorrelationID   = "correlationID"

	paramStatus     = "status"
	paramRuntimeIDs = "runtimeID"
	paramCluster    = "cluster"
)

func startWebserver(ctx context.Context, o *Options) error {
	//routing
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters", paramContractVersion),
		callHandler(o, createOrUpdateCluster)).
		Methods("PUT", "POST")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}", paramContractVersion, paramRuntimeID),
		callHandler(o, deleteCluster)).
		Methods("DELETE")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}", paramContractVersion, paramCluster),
		callHandler(o, deleteCluster)).
		Methods("DELETE")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/configs/{%s}/status", paramContractVersion, paramRuntimeID, paramConfigVersion),
		callHandler(o, getCluster)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/status", paramContractVersion, paramRuntimeID),
		callHandler(o, getLatestCluster)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/status", paramContractVersion, paramRuntimeID),
		callHandler(o, updateLatestCluster)).
		Methods("PUT")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/statusChanges", paramContractVersion, paramRuntimeID), //supports offset-param
		callHandler(o, statusChanges)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/operations/{%s}/callback/{%s}", paramContractVersion, paramSchedulingID, paramCorrelationID),
		callHandler(o, operationCallback)).
		Methods("POST")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/reconciliations", paramContractVersion),
		callHandler(o, getReconciliations)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/reconciliations/{%s}/info", paramContractVersion, paramSchedulingID),
		callHandler(o, getReconciliationInfo)).
		Methods("GET")

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
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Contract version undefined").Error(),
		})
		return
	}
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to read received JSON payload").Error(),
		})
		return
	}
	clusterModel, err := keb.NewModelFactory(contractV).Cluster(reqBody)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to unmarshal JSON payload").Error(),
		})
		return
	}
	if _, err := (&kubernetes.ClientBuilder{}).WithString(clusterModel.Kubeconfig).Build(true); err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "kubeconfig not accepted").Error(),
		})
		return
	}
	clusterState, err := o.Registry.Inventory().CreateOrUpdate(contractV, clusterModel)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to create or update cluster entity").Error(),
		})
		return
	}
	//respond status URL
	sendResponse(w, r, clusterState, o.Registry.ReconciliationRepository())
}

func getCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	runtimeID, err := params.String(paramRuntimeID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	configVersion, err := params.Int64(paramConfigVersion)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	clusterState, err := o.Registry.Inventory().Get(runtimeID, configVersion)
	if err != nil {
		var httpCode int
		if repository.IsNotFoundError(err) {
			httpCode = http.StatusNotFound
		} else {
			httpCode = http.StatusInternalServerError
		}
		server.SendHTTPError(w, httpCode, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Could not retrieve cluster state").Error(),
		})
		return
	}
	sendResponse(w, r, clusterState, o.Registry.ReconciliationRepository())
}

func updateLatestCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	clusterName, err := params.String(paramRuntimeID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	contractV, err := params.Int64(paramContractVersion)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Contract version undefined").Error(),
		})
		return
	}
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to read received JSON payload").Error(),
		})
		return
	}

	status, err := keb.NewModelFactory(contractV).Status(reqBody)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to unmarshal JSON payload").Error(),
		})
		return
	}

	clusterState, err := o.Registry.Inventory().GetLatest(clusterName)
	if err != nil {
		httpCode := http.StatusInternalServerError
		if repository.IsNotFoundError(err) {
			httpCode = http.StatusNotFound
		}
		server.SendHTTPError(w, httpCode, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Could not update cluster state").Error(),
		})
		return
	}

	clusterState, err = o.Registry.Inventory().UpdateStatus(clusterState, model.Status(status.Status))
	if err != nil {
		httpCode := http.StatusInternalServerError
		if repository.IsNotFoundError(err) {
			httpCode = http.StatusNotFound
		}
		server.SendHTTPError(w, httpCode, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Could not update cluster state").Error(),
		})
		return
	}

	sendResponse(w, r, clusterState, o.Registry.ReconciliationRepository())
}

func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func getReconciliations(o *Options, w http.ResponseWriter, r *http.Request) {
	statuses := r.URL.Query()[paramStatus]

	// validate statuseses
	for _, statusStr := range statuses {
		if _, err := keb.ToStatus(statusStr); err != nil {
			server.SendHTTPError(
				w,
				http.StatusBadRequest,
				&keb.BadRequest{Error: err.Error()},
			)
			return
		}
	}

	runtimeIDs := r.URL.Query()[paramRuntimeIDs]

	// Fetch all reconciliation entitlies base on runtime id
	reconciles, err := o.Registry.
		ReconciliationRepository().
		GetReconciliations(
			&reconciliation.WithRuntimeIDs{RuntimeIDs: runtimeIDs},
		)

	if err != nil {
		server.SendHTTPError(
			w,
			http.StatusInternalServerError,
			&keb.InternalError{Error: err.Error()},
		)
		return
	}

	results := []keb.Reconciliation{}

RESULT_LOOP:
	for _, reconcile := range reconciles {
		// FIXME add new method in inventory to fetch multiple statuses via runtimeID and fetch it in 1 go
		state, err := o.Registry.
			Inventory().
			GetLatest(reconcile.RuntimeID)

		if err != nil {
			server.SendHTTPError(
				w,
				http.StatusInternalServerError,
				&keb.InternalError{
					Error: err.Error(),
				})
			return
		}

		if len(statuses) != 0 && !contains(statuses, string(state.Status.Status)) {
			continue RESULT_LOOP
		}

		results = append(results, keb.Reconciliation{
			Created:      reconcile.Created,
			Lock:         reconcile.Lock,
			RuntimeID:    reconcile.RuntimeID,
			SchedulingID: reconcile.SchedulingID,
			Status:       keb.Status(state.Status.Status),
			Updated:      reconcile.Updated,
		})
	}

	//respond
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(keb.ReconcilationsOKResponse(results)); err != nil {
		server.SendHTTPError(
			w,
			http.StatusInternalServerError,
			&reconciler.HTTPErrorResponse{
				Error: errors.Wrap(err, "Failed to encode cluster list response").Error(),
			})
		return
	}
}

func getReconciliationInfo(o *Options, w http.ResponseWriter, r *http.Request) {
	// find arguments
	params := server.NewParams(r)
	schedulingID, err := params.String(paramSchedulingID)
	if err != nil {
		server.SendHTTPError(
			w,
			http.StatusBadRequest,
			&keb.BadRequest{Error: err.Error()},
		)
		return
	}
	// fetch all reconciliation operations for given scheduling id
	operations, err := o.Registry.ReconciliationRepository().GetOperations(schedulingID)
	if err != nil {
		server.SendHTTPError(
			w,
			http.StatusInternalServerError,
			&keb.InternalError{Error: err.Error()},
		)
		return
	}
	// return 404 if no reconciliation opertations found
	operationLen := len(operations)
	if operationLen < 1 {
		server.SendHTTPError(
			w,
			http.StatusNotFound,
			&keb.HTTPErrorResponse{
				Error: fmt.Sprintf("Reconciliation run with schedulingID: '%s' does not exist", schedulingID),
			},
		)
		return
	}
	// find runtime id
	runtimeID := operations[0].RuntimeID
	// fetch cluster latest state
	lastState, err := o.Registry.
		Inventory().
		GetLatest(runtimeID)

	if err != nil || lastState == nil {
		server.SendHTTPError(
			w,
			http.StatusInternalServerError,
			&keb.InternalError{
				Error: fmt.Sprintf(
					"Failed to fetch the lates state for the cluster with runtimeID: '%s'",
					schedulingID,
				),
			},
		)
		return
	}
	// update response with the lates state of the cluster
	result := keb.ReconcilationOperationsOKResponse{
		Cluster: clusterMetadata(runtimeID, lastState),
	}
	// prepare reconciliation operations
	resultOperations := make([]keb.Operation, operationLen)
	for i := 0; i < operationLen; i++ {
		operation := operations[i]

		resultOperations[i] = keb.Operation{
			Component:     operation.Component,
			CorrelationID: operation.CorrelationID,
			Created:       operation.Created,
			Priority:      operation.Priority,
			Reason:        operation.Reason,
			SchedulingID:  operation.CorrelationID,
			State:         string(operation.State),
			Updated:       operation.Updated,
		}
	}
	// update response with the reconciliation operations
	result.Operations = &resultOperations
	//respond
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(keb.ReconcilationOperationsOKResponse(result)); err != nil {
		server.SendHTTPError(
			w,
			http.StatusInternalServerError,
			&keb.InternalError{
				Error: errors.Wrap(err, "Failed to encode cluster list response").Error(),
			})
		return
	}
}

func getLatestCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	runtimeID, err := params.String(paramRuntimeID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	clusterState, err := o.Registry.Inventory().GetLatest(runtimeID)
	if err != nil {
		httpCode := http.StatusInternalServerError
		if repository.IsNotFoundError(err) {
			httpCode = http.StatusNotFound
		}
		server.SendHTTPError(w, httpCode, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Could not retrieve cluster state").Error(),
		})
		return
	}
	sendResponse(w, r, clusterState, o.Registry.ReconciliationRepository())
}

func statusChanges(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)

	runtimeID, err := params.String(paramRuntimeID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}

	offset, err := params.String(paramOffset)
	if err != nil {
		offset = fmt.Sprintf("%dh", 24*7) //default offset is 1 week
	}

	o.Logger().Debugf("Using an offset of '%s' for cluster status updates", offset)

	duration, err := time.ParseDuration(offset)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}

	statusChanges, err := o.Registry.Inventory().StatusChanges(runtimeID, duration)
	if err != nil {
		httpCode := http.StatusInternalServerError
		if repository.IsNotFoundError(err) {
			httpCode = http.StatusNotFound
		}
		server.SendHTTPError(w, httpCode, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Could not retrieve cluster statusChanges").Error(),
		})
		return
	}

	resp := keb.HTTPClusterStatusResponse{}
	for _, statusChange := range statusChanges {
		kebClusterStatus, err := statusChange.Status.GetKEBClusterStatus()
		if err != nil {
			server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
				Error: errors.Wrap(err, "Failed to map reconciler internal cluster status to KEB cluster status").Error(),
			})
			return
		}
		resp.StatusChanges = append(resp.StatusChanges, keb.StatusChange{
			Started:  statusChange.Status.Created,
			Duration: int64(statusChange.Duration),
			Status:   kebClusterStatus,
		})
	}

	//respond
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to encode cluster statusChanges response").Error(),
		})
		return
	}
}

func deleteCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	runtimeID, err := params.String(paramRuntimeID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	if _, err := o.Registry.Inventory().GetLatest(runtimeID); repository.IsNotFoundError(err) {
		server.SendHTTPError(w, http.StatusNotFound, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, fmt.Sprintf("Deletion impossible: Cluster '%s' not found", runtimeID)).Error(),
		})
		return
	}
	state, err := o.Registry.Inventory().MarkForDeletion(runtimeID)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, fmt.Sprintf("Failed to delete cluster '%s'", runtimeID)).Error(),
		})
		return
	}
	sendResponse(w, r, state, o.Registry.ReconciliationRepository())
}

func operationCallback(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	schedulingID, err := params.String(paramSchedulingID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	correlationID, err := params.String(paramCorrelationID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}

	var body reconciler.CallbackMessage
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to read received JSON payload").Error(),
		})
		return
	}

	err = json.Unmarshal(reqBody, &body)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to unmarshal JSON payload").Error(),
		})
		return
	}

	if body.Status == "" {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: fmt.Errorf("status not provided in payload").Error(),
		})
		return
	}

	switch body.Status {
	case reconciler.StatusNotstarted, reconciler.StatusRunning:
		err = updateOperationState(o, schedulingID, correlationID, model.OperationStateInProgress)
	case reconciler.StatusFailed:
		err = updateOperationState(o, schedulingID, correlationID, model.OperationStateFailed, body.Error)
	case reconciler.StatusSuccess:
		err = updateOperationState(o, schedulingID, correlationID, model.OperationStateDone)
	case reconciler.StatusError:
		err = updateOperationState(o, schedulingID, correlationID, model.OperationStateError, body.Error)
	}
	if err != nil {
		httpCode := http.StatusBadRequest
		if repository.IsNotFoundError(err) {
			httpCode = http.StatusNotFound
		}
		server.SendHTTPError(w, httpCode, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
}

func updateOperationState(o *Options, schedulingID, correlationID string, state model.OperationState, reason ...string) error {
	err := o.Registry.ReconciliationRepository().UpdateOperationState(
		schedulingID, correlationID, state, strings.Join(reason, ", "))
	if err != nil {
		o.Logger().Errorf("REST endpoint failed to update operation (schedulingID:%s/correlationID:%s) "+
			"to state '%s': %s", schedulingID, correlationID, state, err)
	}
	return err
}

func sendResponse(w http.ResponseWriter, r *http.Request, clusterState *cluster.State, reconciliationRepository reconciliation.Repository) {
	respModel, err := newClusterResponse(r, clusterState, reconciliationRepository)
	if err != nil {
		server.SendHTTPError(w, 500, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "failed to generate cluster response model").Error(),
		})
		return
	}

	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(respModel); err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to encode response payload to JSON").Error(),
		})
	}
}

func newClusterResponse(r *http.Request, clusterState *cluster.State, reconciliationRepository reconciliation.Repository) (*keb.HTTPClusterResponse, error) {
	kebStatus, err := clusterState.Status.GetKEBClusterStatus()
	if err != nil {
		return nil, err
	}

	var failures []keb.Failure
	if clusterState.Status.Status == model.ClusterStatusReconcileError || clusterState.Status.Status == model.ClusterStatusDeleteError ||
		clusterState.Status.Status == model.ClusterStatusReconciling || clusterState.Status.Status == model.ClusterStatusDeleting {
		reconciliations, err := reconciliationRepository.GetReconciliations(&reconciliation.WithClusterConfigStatus{ClusterConfigStatus: clusterState.Status.ID})
		if err != nil {
			return nil, err
		}
		if len(reconciliations) > 0 {
			operations, err := reconciliationRepository.GetOperations(reconciliations[0].SchedulingID)
			if err != nil {
				return nil, err
			}

			for _, operation := range operations {
				if operation.State.IsError() {
					failures = append(failures, keb.Failure{
						Component: operation.Component,
						Reason:    operation.Reason,
					})
				}
			}
		}
	}

	return &keb.HTTPClusterResponse{
		Cluster:              clusterState.Cluster.RuntimeID,
		ClusterVersion:       clusterState.Cluster.Version,
		ConfigurationVersion: clusterState.Configuration.Version,
		Status:               kebStatus,
		Failures:             &failures,
		StatusURL: (&url.URL{
			Scheme: viper.GetString("mothership.scheme"),
			Host:   fmt.Sprintf("%s:%s", viper.GetString("mothership.host"), viper.GetString("mothership.port")),
			Path: func() string {
				apiVersion := strings.Split(r.URL.RequestURI(), "/")[1]
				return fmt.Sprintf("%s/clusters/%s/configs/%d/status", apiVersion,
					clusterState.Cluster.RuntimeID, clusterState.Configuration.Version)
			}(),
		}).String(),
	}, nil
}
