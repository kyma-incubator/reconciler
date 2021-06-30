package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/spf13/cobra"
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
	router.HandleFunc("/clusters",
		callHandler(o, createOrUpdate)).
		Methods("PUT", "POST")

	router.HandleFunc("/clusters/{cluster}",
		callHandler(o, delete)).
		Methods("DELETE")

	router.HandleFunc("/clusters/{cluster}/configs/{configVersion}/status",
		callHandler(o, get)).
		Methods("GET")

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
	reqBody, _ := ioutil.ReadAll(r.Body)
	var clusterPayload cluster.Cluster
	if err := json.Unmarshal(reqBody, &clusterPayload); err != nil {
		sendError(w)
	}
	clusterState, err := o.Inventory().CreateOrUpdate(&clusterPayload)
	if err != nil {
		sendError(w)
	}
	url := fmt.Sprintf("%s%s/%s/configs/%d/status", r.Host, r.URL.RequestURI(), clusterState.Cluster.Cluster, clusterState.Configuration.Version)
	if err := json.NewEncoder(w).Encode(url); err != nil {
		sendError(w)
	}
}

func get(o *Options, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cluster := strings.TrimSpace(vars["cluster"])
	if cluster == "" {
		sendError(w)
	}
	configVersion, err := strconv.ParseInt(vars["configVersion"], 10, 64)
	if err != nil {
		sendError(w)
	}
	clusterState, err := o.Inventory().Get(cluster, configVersion)
	if err != nil {
		sendError(w)
	}
	if err := json.NewEncoder(w).Encode(clusterState.Status.Status); err != nil {
		sendError(w)
	}
}

func delete(o *Options, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cluster := strings.TrimSpace(vars["cluster"])
	if cluster == "" {
		sendError(w)
	}
	if err := o.Inventory().Delete(cluster); err != nil {
		sendError(w)
	}
}

func sendError(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
