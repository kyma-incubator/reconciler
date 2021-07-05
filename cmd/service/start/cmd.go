package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/pkg/errors"
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
	//listen on os events
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	//create context
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		oscall := <-c
		if oscall == os.Interrupt {
			cancel()
		}
	}()

	//run webserver within context
	var err error
	if err = runServer(ctx, o); err != nil {
		o.Logger().Error(fmt.Sprintf("Failed to run webserver: %s", err))
	}
	return err
}

func runServer(ctx context.Context, o *Options) error {
	o.Logger().Info(fmt.Sprintf("Webserver starting and listening on port %d", o.Port))
	srv := startServer(o)
	<-ctx.Done()
	o.Logger().Info("Webserver stopping")
	return stopServer(o, srv)
}

func startServer(o *Options) *http.Server {
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

	//start server
	srv := &http.Server{Addr: fmt.Sprintf(":%d", o.Port), Handler: router}
	go func() {
		var err error
		if o.SSLSupport() {
			err = srv.ListenAndServeTLS(o.SSLCrt, o.SSLKey)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			o.Logger().Error(fmt.Sprintf("Webserver startup failed: %s", err))
		}
	}()
	return srv
}

func stopServer(o *Options, srv *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	err := srv.Shutdown(ctx)

	if err == nil {
		o.Logger().Info("Webserver gracefully stopped")
	} else {
		o.Logger().Error(fmt.Sprintf("Webserver shutdown failed: %s", err))
	}
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
		sendError(w, http.StatusBadRequest, errors.Wrap(err, "Contract version undefined"))
		return
	}
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to read received JSON payload"))
		return
	}
	clusterPayload, err := keb.NewModelFactory(contractV).Cluster(reqBody)
	if err != nil {
		sendError(w, http.StatusBadRequest, errors.Wrap(err, "Failed to unmarshal JSON payload"))
		return
	}
	clusterState, err := o.Inventory().CreateOrUpdate(contractV, clusterPayload)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to create or update cluster entity"))
		return
	}
	//respond status URL
	response := response(clusterState)
	response["statusUrl"] = fmt.Sprintf("%s%s/%s/configs/%d/status", r.Host, r.URL.RequestURI(), clusterState.Cluster.Cluster, clusterState.Configuration.Version)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to generate progress URL response"))
		return
	}
}

func get(o *Options, w http.ResponseWriter, r *http.Request) {
	params := newParam(r)
	cluster, err := params.string("cluster")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	configVersion, err := params.int64("configVersion")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	clusterState, err := o.Inventory().Get(cluster, configVersion)
	if err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Cloud not retrieve cluster state"))
		return
	}
	//respond
	if err := json.NewEncoder(w).Encode(response(clusterState)); err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, "Failed to encode cluster status response"))
		return
	}
}

func delete(o *Options, w http.ResponseWriter, r *http.Request) {
	params := newParam(r)
	cluster, err := params.string("cluster")
	if err != nil {
		sendError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := o.Inventory().GetLatest(cluster); repository.IsNotFoundError(err) {
		sendError(w, http.StatusNotFound, errors.Wrap(err, fmt.Sprintf("Deletion impossible: cluster '%s' not found", cluster)))
		return
	}
	if err := o.Inventory().Delete(cluster); err != nil {
		sendError(w, http.StatusInternalServerError, errors.Wrap(err, fmt.Sprintf("Failed to delete cluster '%s'", cluster)))
		return
	}
}

func response(clusterState *cluster.State) map[string]interface{} {
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
