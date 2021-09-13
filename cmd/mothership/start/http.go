package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/scheduler"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/kubernetes"

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

const (
	paramContractVersion = "contractVersion"
	paramCluster         = "cluster"
	paramConfigVersion   = "configVersion"
	paramOffset          = "offset"
	paramSchedulingID    = "schedulingID"
	paramCorrelationID   = "correlationID"
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
		fmt.Sprintf("/v{%s}/clusters/{%s}/statusChanges", paramContractVersion, paramCluster), //supports offset-param
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
	sendResponse(w, r, clusterState)
}

func getCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	clusterName, err := params.String(paramCluster)
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
	clusterState, err := o.Registry.Inventory().Get(clusterName, configVersion)
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
	sendResponse(w, r, clusterState)
}

func getLatestCluster(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	clusterName, err := params.String(paramCluster)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
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
			Error: errors.Wrap(err, "Could not retrieve cluster state").Error(),
		})
		return
	}
	sendResponse(w, r, clusterState)
}

func statusChanges(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)

	clusterName, err := params.String(paramCluster)
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

	statusChanges, err := o.Registry.Inventory().StatusChanges(clusterName, duration)
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
		resp.StatusChanges = append(resp.StatusChanges, &keb.StatusChange{
			Started:  statusChange.Status.Created,
			Duration: statusChange.Duration,
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
	clusterName, err := params.String(paramCluster)
	if err != nil {
		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
	if _, err := o.Registry.Inventory().GetLatest(clusterName); repository.IsNotFoundError(err) {
		server.SendHTTPError(w, http.StatusNotFound, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, fmt.Sprintf("Deletion impossible: Cluster '%s' not found", clusterName)).Error(),
		})
		return
	}
	if err := o.Registry.Inventory().Delete(clusterName); err != nil {
		server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, fmt.Sprintf("Failed to delete cluster '%s'", clusterName)).Error(),
		})
		return
	}
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
	case reconciler.NotStarted, reconciler.Running:
		err = o.Registry.OperationsRegistry().SetInProgress(correlationID, schedulingID)
	case reconciler.Failed:
		err = o.Registry.OperationsRegistry().SetFailed(correlationID, schedulingID,
			fmt.Sprintf("Reconciler reported failure status: %s", body.Error.Error()))
	case reconciler.Success:
		err = o.Registry.OperationsRegistry().SetDone(correlationID, schedulingID)
	case reconciler.Error:
		err = o.Registry.OperationsRegistry().SetError(correlationID, schedulingID,
			fmt.Sprintf("Reconciler reported error status: %s", body.Error.Error()))
	}
	if err != nil {
		httpCode := http.StatusBadRequest
		if scheduler.IsOperationNotFoundError(err) {
			httpCode = http.StatusNotFound
		}
		server.SendHTTPError(w, httpCode, &reconciler.HTTPErrorResponse{
			Error: err.Error(),
		})
		return
	}
}

func sendResponse(w http.ResponseWriter, r *http.Request, clusterState *cluster.State) {
	respModel, err := newClusterResponse(r, clusterState)
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

func newClusterResponse(r *http.Request, clusterState *cluster.State) (*keb.HTTPClusterResponse, error) {
	kebStatus, err := clusterState.Status.GetKEBClusterStatus()
	if err != nil {
		return nil, err
	}

	return &keb.HTTPClusterResponse{
		Cluster:              clusterState.Cluster.Cluster,
		ClusterVersion:       clusterState.Cluster.Version,
		ConfigurationVersion: clusterState.Configuration.Version,
		Status:               kebStatus,
		StatusURL: fmt.Sprintf("%s%s/%s/configs/%d/status", r.Host, r.URL.RequestURI(),
			clusterState.Cluster.Cluster, clusterState.Configuration.Version),
	}, nil
}
