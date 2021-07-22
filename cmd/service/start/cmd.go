package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

const (
	paramContractVersion = "contractVersion"
	paramCluster         = "cluster"
	paramConfigVersion   = "configVersion"
	paramOffset          = "offset"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reconciler service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o)
		},
	}
	cmd.Flags().IntVar(&o.Port, "port", 8080, "Webserver port")
	cmd.Flags().StringVar(&o.SSLCrt, "crt", "", "Path to SSL certificate file")
	cmd.Flags().StringVar(&o.SSLKey, "key", "", "Path to SSL key file")
	return cmd
}

func Run(o *Options) error {
	ctx := cli.NewContext()
	//run webserver within context
	return startWebserver(ctx, o)
}

func startWebserver(ctx context.Context, o *Options) error {
	//routing
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters", paramContractVersion),
		callHandler(o, createOrUpdate)).
		Methods("PUT", "POST")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}", paramContractVersion, paramCluster),
		callHandler(o, delete)).
		Methods("DELETE")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/configs/{%s}/status", paramContractVersion, paramCluster, paramConfigVersion),
		callHandler(o, get)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/status", paramContractVersion, paramCluster),
		callHandler(o, getLatest)).
		Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v{%s}/clusters/{%s}/statusChanges/{%s}", paramContractVersion, paramCluster, paramOffset),
		callHandler(o, statusChanges)).
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

func createOrUpdate(o *Options, w http.ResponseWriter, r *http.Request) {
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

func get(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	cl, err := params.String("cluster")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	configVersion, err := params.Int64("configVersion")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	clusterState, err := o.Registry.Inventory().Get(cl, configVersion)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Cloud not retrieve cluster state"))
		return
	}
	sendResponse(w, responsePayload(clusterState))
}

func getLatest(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	cl, err := params.String("cluster")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	clusterState, err := o.Registry.Inventory().GetLatest(cl)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Could not retrieve cluster state"))
		return
	}
	sendResponse(w, responsePayload(clusterState))
}

func statusChanges(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	cl, err := params.String("cluster")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	offset, err := params.String("offset")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	duration, err := time.ParseDuration(offset)
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	changes, err := o.Registry.Inventory().StatusChanges(cl, duration)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Cloud not retrieve cluster statusChanges"))
		return
	}
	//respond
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(changes); err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to encode cluster statusChanges response"))
		return
	}
}

func delete(o *Options, w http.ResponseWriter, r *http.Request) {
	params := server.NewParams(r)
	cl, err := params.String("cluster")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := o.Registry.Inventory().GetLatest(cl); repository.IsNotFoundError(err) {
		sendError(w, http.StatusNotFound, errors.Wrap(err, fmt.Sprintf("Deletion impossible: cluster '%s' not found", cl)))
		return
	}
	if err := o.Registry.Inventory().Delete(cl); err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, fmt.Sprintf("Failed to delete cluster '%s'", cl)))
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
