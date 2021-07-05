package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

const (
	paramContractVersion = "contractVersion"
	paramCluster         = "cluster"
	paramConfigVersion   = "configVersion"
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
	o.Logger().Info(fmt.Sprintf("Starting webserver on port %d", o.Port))

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

	//metrics endpoint
	metrics.RegisterAll(o.Inventory(), o.Logger())
	router.Handle("/metrics", promhttp.Handler())

	//start server
	var err error
	addr := fmt.Sprintf(":%d", o.Port)
	if o.SSLSupport() {
		err = http.ListenAndServeTLS(addr, o.SSLCrt, o.SSLKey, router)
	} else {
		err = http.ListenAndServe(addr, router)
	}

	o.Logger().Info("Webserver stopped")
	return err
}

func callHandler(o *Options, handler func(o *Options, w http.ResponseWriter, r *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(o, w, r)
	}
}

func createOrUpdate(o *Options, w http.ResponseWriter, r *http.Request) {
	params := newParam(r)
	contractV, err := params.int64(paramContractVersion)
	if err != nil {
		sendError(w, errors.Wrap(err, "Contract version undefined"))
	}
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, errors.Wrap(err, "Failed to read received JSON payload"))
	}
	clusterPayload, err := keb.NewModelFactory(contractV).Cluster(reqBody)
	if err != nil {
		sendError(w, errors.Wrap(err, "Failed to unmarshal JSON payload"))
	}
	clusterState, err := o.Inventory().CreateOrUpdate(contractV, clusterPayload)
	if err != nil {
		sendError(w, errors.Wrap(err, "Failed to create or update cluster entity"))
	}
	url := fmt.Sprintf("%s%s/%s/configs/%d/status", r.Host, r.URL.RequestURI(), clusterState.Cluster.Cluster, clusterState.Configuration.Version)
	if err := json.NewEncoder(w).Encode(url); err != nil {
		sendError(w, errors.Wrap(err, "Failed to generate progress URL response"))
	}
}

func get(o *Options, w http.ResponseWriter, r *http.Request) {
	params := newParam(r)
	cluster, err := params.string("cluster")
	if err != nil {
		sendError(w, err)
	}
	configVersion, err := params.int64("configVersion")
	if err != nil {
		sendError(w, err)
	}
	clusterState, err := o.Inventory().Get(cluster, configVersion)
	if err != nil {
		sendError(w, errors.Wrap(err, "Cloud not retrieve cluster state"))
	}
	if err := json.NewEncoder(w).Encode(clusterState.Status.Status); err != nil {
		sendError(w, errors.Wrap(err, "Failed to encode cluster status response"))
	}
}

func delete(o *Options, w http.ResponseWriter, r *http.Request) {
	params := newParam(r)
	cluster, err := params.string("cluster")
	if err != nil {
		sendError(w, err)
	}
	if err := o.Inventory().Delete(cluster); err != nil {
		sendError(w, errors.Wrap(err, fmt.Sprintf("Failed to delete cluster '%s'", cluster)))
	}
}

func sendError(w http.ResponseWriter, err error) {
	http.Error(w, fmt.Sprintf("%s\n\n%s", http.StatusText(http.StatusInternalServerError), err.Error()), http.StatusInternalServerError)
}

type param struct {
	params map[string]string
}

func newParam(r *http.Request) *param {
	return &param{
		params: mux.Vars(r),
	}
}
func (p *param) string(name string) (string, error) {
	result, ok := p.params[name]
	if !ok {
		return "", fmt.Errorf("Parameter '%s' undefined", name)
	}
	return result, nil
}

func (p *param) int64(name string) (int64, error) {
	strResult, err := p.string(name)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strResult, 10, 64)
}
