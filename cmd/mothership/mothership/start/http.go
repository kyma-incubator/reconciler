package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/internal/converters"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
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
	paramBefore     = "before"
	paramAfter      = "after"
	paramLast       = "last"
	paramTimeFormat = time.RFC3339
	paramPoolID     = "poolID"
)

func startWebserver(ctx context.Context, o *Options) error {
	//routing
	mainRouter := mux.NewRouter()
	apiRouter := mainRouter.PathPrefix("/").Subrouter()
	metricsRouter := mainRouter.Path("/metrics").Subrouter()
	healthRouter := mainRouter.PathPrefix("/health").Subrouter()

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/operations/{%s}/{%s}/stop", paramContractVersion, paramSchedulingID, paramCorrelationID),
		callHandler(o, updateOperationStatus)).
		Methods(http.MethodPost)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters", paramContractVersion),
		callHandler(o, createOrUpdateCluster)).
		Methods(http.MethodPost, http.MethodPut)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}", paramContractVersion, paramRuntimeID),
		callHandler(o, deleteCluster)).
		Methods(http.MethodDelete)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%v}/clusters/state", paramContractVersion),
		callHandler(o, getClustersState)).
		Methods(http.MethodGet)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/configs/{%s}/status", paramContractVersion, paramRuntimeID, paramConfigVersion),
		callHandler(o, getCluster)).
		Methods(http.MethodGet)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/status", paramContractVersion, paramRuntimeID),
		callHandler(o, getLatestCluster)).
		Methods(http.MethodGet)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/status", paramContractVersion, paramRuntimeID),
		callHandler(o, updateLatestCluster)).
		Methods(http.MethodPut)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/statusChanges", paramContractVersion, paramRuntimeID), //supports offset-param
		callHandler(o, statusChanges)).
		Methods(http.MethodGet)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/operations/{%s}/callback/{%s}", paramContractVersion, paramSchedulingID, paramCorrelationID),
		callHandler(o, operationCallback)).
		Methods(http.MethodPost)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/operations/{%s}/callback/{%s}/processingDuration", paramContractVersion, paramSchedulingID, paramCorrelationID),
		callHandler(o, updateOperationProcessingDuration)).
		Methods(http.MethodPost)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/reconciliations", paramContractVersion),
		callHandler(o, getReconciliations)).
		Methods(http.MethodGet)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/reconciliations/{%s}/info", paramContractVersion, paramSchedulingID),
		callHandler(o, getReconciliationInfo)).
		Methods(http.MethodGet)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/config/{%s}", paramContractVersion, paramRuntimeID, paramConfigVersion),
		callHandler(o, getKymaConfig)).Methods(http.MethodGet)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/occupancy/{%s}", paramContractVersion, paramPoolID),
		callHandler(o, deleteComponentWorkerPoolOccupancy)).Methods(http.MethodDelete)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/occupancy/{%s}", paramContractVersion, paramPoolID),
		callHandler(o, createOrUpdateComponentWorkerPoolOccupancy)).Methods(http.MethodPost)

	//metrics endpoint
	if o.OccupancyTracking {
		metrics.RegisterOccupancy(o.Registry.OccupancyRepository(), o.ReconcilerList, o.Logger())
	}
	metrics.RegisterWaitingAndNotReadyReconciliations(o.Registry.Inventory(), o.Logger())
	metrics.RegisterProcessingDuration(o.Registry.ReconciliationRepository(), o.ReconcilerList, o.Logger())
	metricsRouter.Handle("", promhttp.Handler())

	//liveness and readiness checks
	healthRouter.HandleFunc("/live", live)
	healthRouter.HandleFunc("/ready", ready(o))

	if o.AuditLog && o.AuditLogFile != "" && o.AuditLogTenantID != "" {
		auditLogger, err := NewLoggerWithFile(o.AuditLogFile)
		if err != nil {
			return err
		}
		defer func() { _ = auditLogger.Sync() }() // make golint happy
		auditLoggerMiddelware := newAuditLoggerMiddelware(auditLogger, o)
		apiRouter.Use(auditLoggerMiddelware)
	}
	//start server process
	srv := &server.Webserver{
		Logger:     o.Logger(),
		Port:       o.Port,
		SSLCrtFile: o.SSLCrt,
		SSLKeyFile: o.SSLKey,
		Router:     mainRouter,
	}
	return srv.Start(ctx) //blocking call
}

func live(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func ready(o *Options) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if o.Registry.Connection().Ping() != nil {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
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

	clusterStateOld, err := o.Registry.Inventory().GetLatest(clusterModel.RuntimeID)
	if err != nil && !repository.IsNotFoundError(err) {
		server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to get latest status").Error(),
		})
		return
	}

	clusterStateNew, err := o.Registry.Inventory().CreateOrUpdate(contractV, clusterModel)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to create or update cluster entity").Error(),
		})
		return
	}

	if clusterStateOld != nil && clusterStateOld.Status.Status.IsDisabled() {
		if clusterStateNew, err = o.Registry.Inventory().UpdateStatus(clusterStateNew, model.ClusterStatusReconcileDisabled); err != nil {
			server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
				Error: errors.Wrap(err, "Failed to disable cluster after an update").Error(),
			})
			return
		}
	}

	//respond status URL
	sendResponse(w, r, clusterStateNew, o.Registry.ReconciliationRepository())
}

func getClustersState(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)

	if runtimeID, err := params.String(paramRuntimeID); err == nil && runtimeID != "" {
		state, err := o.Registry.Inventory().GetLatest(runtimeID)
		if err != nil {
			server.SendHTTPError(w, http.StatusNotFound, &keb.HTTPErrorResponse{
				Error: errors.Wrap(err, "Failed to get cluster state based on runtimeID").Error(),
			})
			return
		}

		sendClusterStateResponse(w, state)
		return
	}

	filters := []operation.Filter{}
	if schedulingID, err := params.String(paramSchedulingID); err == nil && schedulingID != "" {
		filters = append(filters, &operation.WithSchedulingID{
			SchedulingID: schedulingID,
		})
	}

	if correlationID, err := params.String(paramCorrelationID); err == nil && correlationID != "" {
		filters = append(filters, &operation.WithCorrelationID{
			CorrelationID: correlationID,
		})
	}

	operations, err := o.Registry.ReconciliationRepository().GetOperations(&operation.FilterMixer{
		Filters: append(filters, &operation.Limit{Count: 1}),
	})
	if err != nil {
		server.SendHTTPError(w, http.StatusNotFound, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	} else if len(operations) < 1 {
		server.SendHTTPError(w, http.StatusNotFound, &keb.HTTPErrorResponse{
			Error: "Failed to get operations correlated to cluster state based on given parameters",
		})
		return
	}

	runtimeID := operations[0].RuntimeID
	clusterConfig := operations[0].ClusterConfig
	state, err := o.Registry.Inventory().Get(runtimeID, clusterConfig)
	if err != nil {
		server.SendHTTPError(w, http.StatusNotFound, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}

	sendClusterStateResponse(w, state)
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

func getReconciliations(o *Options, w http.ResponseWriter, r *http.Request) {
	// define variables
	var filters []reconciliation.Filter

	params := server.NewParams(r)

	if runtimeIDs, err := params.StrSlice(paramRuntimeIDs); err == nil {
		filters = append(filters, &reconciliation.WithRuntimeIDs{RuntimeIDs: runtimeIDs})
	}

	if statuses, err := params.StrSlice(paramStatus); err == nil {
		if err := validateStatuses(statuses); err != nil {
			server.SendHTTPError(w, http.StatusBadRequest, &keb.BadRequest{Error: err.Error()})
			return
		}
		filters = append(filters, &reconciliation.WithStatuses{Statuses: statuses})
	}

	if after, err := params.String(paramAfter); err == nil && after != "" {
		t, err := time.Parse(paramTimeFormat, after)
		if err != nil {
			server.SendHTTPError(w, http.StatusBadRequest, &keb.BadRequest{Error: err.Error()})
			return
		}
		filters = append(filters, &reconciliation.WithCreationDateAfter{Time: t})
	}

	if before, err := params.String(paramBefore); err == nil && before != "" {
		t, err := time.Parse(paramTimeFormat, before)
		if err != nil {
			server.SendHTTPError(w, http.StatusBadRequest, &keb.BadRequest{Error: err.Error()})
			return
		}
		filters = append(filters, &reconciliation.WithCreationDateBefore{Time: t})
	}

	if limit, err := params.Int(paramLast); err == nil {
		if err != nil {
			server.SendHTTPError(w, http.StatusBadRequest, &keb.BadRequest{Error: err.Error()})
			return
		}
		filters = append(filters, &reconciliation.Limit{Count: limit})
	}

	// Fetch all reconciliation entities
	reconciles, err := o.Registry.
		ReconciliationRepository().
		GetReconciliations(
			&reconciliation.FilterMixer{Filters: filters},
		)

	if err != nil {
		server.SendHTTPError(
			w,
			http.StatusInternalServerError,
			&keb.InternalError{Error: err.Error()},
		)
		return
	}

	var results []keb.Reconciliation

	for _, reconcile := range reconciles {
		results = append(results, keb.Reconciliation{
			Created:      reconcile.Created,
			Lock:         reconcile.Lock,
			RuntimeID:    reconcile.RuntimeID,
			SchedulingID: reconcile.SchedulingID,
			Status:       keb.Status(reconcile.Status),
			Updated:      reconcile.Updated,
			Finished:     reconcile.Finished,
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
		server.SendHTTPError(w, http.StatusBadRequest, &keb.BadRequest{Error: err.Error()})
		return
	}
	reconciliationEntity, err := o.Registry.ReconciliationRepository().GetReconciliation(schedulingID)
	if err != nil {
		server.SendHTTPErrorMap(w, err)
		return
	}

	operations, err := o.Registry.ReconciliationRepository().GetOperations(&operation.WithSchedulingID{
		SchedulingID: schedulingID,
	})
	if err != nil {
		server.SendHTTPErrorMap(w, err)
		return
	}

	result, err := converters.ConvertReconciliation(reconciliationEntity, operations)
	if err != nil {
		server.SendHTTPErrorMap(w, err)
		return
	}

	//respond
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(keb.ReconciliationInfoOKResponse(result)); err != nil {
		server.SendHTTPErrorMap(w, errors.Wrap(err, "Failed to encode cluster list response"))
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

func updateOperationStatus(o *Options, w http.ResponseWriter, r *http.Request) {
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

	var stopOperation keb.OperationStop
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to read received JSON payload").Error(),
		})
		return
	}

	err = json.Unmarshal(reqBody, &stopOperation)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to unmarshal JSON payload").Error(),
		})
		return
	}

	op, err := getOperationStatus(o, schedulingID, correlationID)
	if err != nil {
		if repository.IsNotFoundError(err) {
			server.SendHTTPError(w, http.StatusNotFound, &reconciler.HTTPErrorResponse{
				Error: "Couldn't find operation",
			})
			return
		}

		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to get operation").Error(),
		})
		return
	}

	if op.State != model.OperationStateNew {
		server.SendHTTPError(w, http.StatusForbidden, &reconciler.HTTPErrorResponse{
			Error: fmt.Sprintf("Operation is in status: %s. Should be in: %s in order to stop it.", op.State, model.OperationStateNew),
		})
		return
	}

	err = updateOperationState(o, schedulingID, correlationID, model.OperationStateDone, stopOperation.Reason)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "while updating operation status").Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
}

func updateOperationProcessingDuration(o *Options, w http.ResponseWriter, r *http.Request) {
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

	var processingDuration reconciler.ProcessingDuration
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to read received JSON payload").Error(),
		})
		return
	}

	err = json.Unmarshal(reqBody, &processingDuration)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to unmarshal JSON payload").Error(),
		})
		return
	}

	err = o.Registry.ReconciliationRepository().UpdateComponentOperationProcessingDuration(schedulingID, correlationID, processingDuration.Duration)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "while updating operation processing duration").Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
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
		err = updateOperationStateAndRetryID(o, schedulingID, correlationID, body.RetryID, model.OperationStateInProgress)
	case reconciler.StatusFailed:
		err = updateOperationStateAndRetryID(o, schedulingID, correlationID, body.RetryID, model.OperationStateFailed, body.Error)
	case reconciler.StatusSuccess:
		err = updateOperationStateAndRetryID(o, schedulingID, correlationID, body.RetryID, model.OperationStateDone)
	case reconciler.StatusError:
		err = updateOperationStateAndRetryID(o, schedulingID, correlationID, body.RetryID, model.OperationStateError, body.Error)
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

func getKymaConfig(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	runtimeID, err := params.String(paramRuntimeID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{Error: err.Error()})
		return
	}

	configVersion, err := params.Int64(paramConfigVersion)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{Error: err.Error()})
		return
	}

	state, err := o.Registry.Inventory().Get(runtimeID, configVersion)
	if err != nil {
		server.SendHTTPErrorMap(w, err)
		return
	}
	if state.Configuration == nil {
		server.SendHTTPErrorMap(w, errors.New("state configuration is nil"))
		return
	}
	response := converters.ConvertConfig(*state.Configuration)

	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "Failed to encode response payload to JSON").Error(),
		})
	}
}

func createOrUpdateComponentWorkerPoolOccupancy(o *Options, w http.ResponseWriter, r *http.Request) {

	params := server.NewParams(r)
	poolID, err := params.String(paramPoolID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	_, err = uuid.Parse(poolID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	var body reconciler.HTTPOccupancyRequest
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
	created, err := o.Registry.OccupancyRepository().CreateOrUpdateWorkerPoolOccupancy(poolID, body.Component, body.RunningWorkers, body.PoolSize)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	if created {
		w.WriteHeader(http.StatusCreated)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func deleteComponentWorkerPoolOccupancy(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	poolID, err := params.String(paramPoolID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	_, err = uuid.Parse(poolID)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	err = o.Registry.OccupancyRepository().RemoveWorkerPoolOccupancy(poolID)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func updateOperationState(o *Options, schedulingID, correlationID string, state model.OperationState, reason ...string) error {
	err := o.Registry.ReconciliationRepository().UpdateOperationState(schedulingID, correlationID, state, true, strings.Join(reason, ", "))
	if err != nil {
		o.Logger().Errorf("REST endpoint failed to update operation (schedulingID:%s/correlationID:%s) "+
			"to state '%s': %s", schedulingID, correlationID, state, err)
	}
	return err
}

func updateOperationStateAndRetryID(o *Options, schedulingID, correlationID, retryID string, state model.OperationState, reason ...string) error {
	err := updateOperationState(o, schedulingID, correlationID, state, reason...)
	if err != nil {
		return err
	}
	err = o.Registry.ReconciliationRepository().UpdateOperationRetryID(schedulingID, correlationID, retryID)
	if err != nil {
		o.Logger().Errorf("REST endpoint failed to update operation (schedulingID:%s/correlationID:%s) "+
			"retryID '%s': %s", schedulingID, correlationID, retryID, err)
	}
	return err
}

func getOperationStatus(o *Options, schedulingID, correlationID string) (*model.OperationEntity, error) {
	op, err := o.Registry.ReconciliationRepository().GetOperation(schedulingID, correlationID)
	if err != nil {
		return nil, errors.Wrap(err, "while getting operation status")
	}
	return op, err
}

func sendResponse(w http.ResponseWriter, r *http.Request, clusterState *cluster.State, reconciliationRepository reconciliation.Repository) {
	respModel, err := newClusterResponse(r, clusterState, reconciliationRepository)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
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

func sendClusterStateResponse(w http.ResponseWriter, state *cluster.State) {
	respModel, err := newClusterStateResponse(state)
	if err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
			Error: errors.Wrap(err, "failed to generate cluster state response model").Error(),
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
			operations, err := reconciliationRepository.GetOperations(&operation.WithSchedulingID{
				SchedulingID: reconciliations[0].SchedulingID,
			})
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

func newClusterStateResponse(state *cluster.State) (*keb.HTTPClusterStateResponse, error) {
	var metadata keb.Metadata
	if state.Cluster.Metadata != nil {
		metadata = keb.Metadata{
			GlobalAccountID: state.Cluster.Metadata.GlobalAccountID,
			InstanceID:      state.Cluster.Metadata.InstanceID,
			Region:          state.Cluster.Metadata.Region,
			ServiceID:       state.Cluster.Metadata.ServiceID,
			ServicePlanID:   state.Cluster.Metadata.ServicePlanID,
			ServicePlanName: state.Cluster.Metadata.ServicePlanName,
			ShootName:       state.Cluster.Metadata.ShootName,
			SubAccountID:    state.Cluster.Metadata.SubAccountID,
		}
	}

	var runtimeInput keb.RuntimeInput
	if state.Cluster.Runtime != nil {
		runtimeInput = keb.RuntimeInput{
			Description: state.Cluster.Runtime.Description,
			Name:        state.Cluster.Runtime.Name,
		}
	}

	components := []keb.Component{}
	for i := range state.Configuration.Components {
		comp := state.Configuration.Components[i]
		configs := []keb.Configuration{}
		for i := range comp.Configuration {
			configs = append(configs, keb.Configuration{
				Key:    comp.Configuration[i].Key,
				Secret: comp.Configuration[i].Secret,
				Value:  comp.Configuration[i].Value,
			})
		}

		components = append(components, keb.Component{
			URL:           comp.URL,
			Component:     comp.Component,
			Configuration: configs,
			Namespace:     comp.Namespace,
			Version:       comp.Version,
		})
	}

	kebStatus, err := state.Status.GetKEBClusterStatus()
	if err != nil {
		return nil, err
	}

	return &keb.HTTPClusterStateResponse{
		Cluster: keb.ClusterState{
			Contract:  &state.Cluster.Contract,
			Created:   &state.Cluster.Created,
			Metadata:  &metadata,
			Runtime:   &runtimeInput,
			RuntimeID: &state.Cluster.RuntimeID,
			Version:   &state.Cluster.Version,
		},
		Configuration: keb.ClusterStateConfiguration{
			Administrators: &state.Configuration.Administrators,
			ClusterVersion: &state.Configuration.ClusterVersion,
			Components:     &components,
			Contract:       &state.Configuration.Contract,
			Created:        &state.Configuration.Created,
			Deleted:        &state.Configuration.Deleted,
			KymaProfile:    &state.Configuration.KymaProfile,
			KymaVersion:    &state.Configuration.KymaVersion,
			RuntimeID:      &state.Configuration.RuntimeID,
			Version:        &state.Configuration.Version,
		},
		Status: keb.ClusterStateStatus{
			ClusterVersion: &state.Status.ClusterVersion,
			ConfigVersion:  &state.Status.ConfigVersion,
			Created:        &state.Status.Created,
			Deleted:        &state.Status.Deleted,
			Id:             &state.Status.ID,
			RuntimeID:      &state.Status.RuntimeID,
			Status:         &kebStatus,
		},
	}, nil
}
