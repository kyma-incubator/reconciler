package compreconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/chart"
	"github.com/kyma-incubator/reconciler/pkg/server"
)

const (
	paramContractVersion  = "version"
	defaultServerPort     = 8080
	defaultMaxRetries     = 5
	defaultUpdateInterval = 30 * time.Second
)

type Action interface {
	Run(version string, kubeClient *kubernetes.Clientset) error
}

type ComponentReconciler struct {
	debug             bool
	serverOpts        serverOpts
	preInstallAction  Action
	installAction     Action
	postInstallAction Action
	chartProvider     *chart.Provider
	updateInterval    time.Duration
	maxRetries        int
}

type serverOpts struct {
	port       int
	sslCrtFile string
	sslKeyFile string
}

func NewComponentReconciler(chartProvider *chart.Provider) *ComponentReconciler {
	return &ComponentReconciler{
		chartProvider: chartProvider,
	}
}

func (r *ComponentReconciler) validate() {
	if r.updateInterval <= 0 {
		r.updateInterval = defaultUpdateInterval
	}
	if r.maxRetries <= 0 {
		r.maxRetries = defaultMaxRetries
	}
	if r.serverOpts.port <= 0 {
		r.serverOpts.port = defaultServerPort
	}
}

func (r *ComponentReconciler) Configure(updateInterval time.Duration, maxRetries int) *ComponentReconciler {
	r.updateInterval = updateInterval
	r.maxRetries = maxRetries
	return r
}

func (r *ComponentReconciler) WithServerConfiguration(port int, sslCrtFile, sslKeyFile string) *ComponentReconciler {
	r.serverOpts.port = port
	r.serverOpts.sslCrtFile = sslCrtFile
	r.serverOpts.sslKeyFile = sslKeyFile
	return r
}

func (r *ComponentReconciler) WithPreInstallAction(preInstallAction Action) *ComponentReconciler {
	r.preInstallAction = preInstallAction
	return r
}

func (r *ComponentReconciler) WithInstallAction(installAction Action) *ComponentReconciler {
	r.installAction = installAction
	return r
}

func (r *ComponentReconciler) WithPostInstallAction(postInstallAction Action) *ComponentReconciler {
	r.postInstallAction = postInstallAction
	return r
}

func (r *ComponentReconciler) Debug() *ComponentReconciler {
	r.debug = true
	return r
}

func (r *ComponentReconciler) StartLocal(ctx context.Context, model *Reconciliation) error {
	r.validate()

	localCbh, err := newLocalCallbackHandler(model.CallbackFct, r.debug)
	if err != nil {
		return err
	}

	return r.start(ctx, model, localCbh)
}

func (r *ComponentReconciler) StartRemote(ctx context.Context) error {
	r.validate()

	//create webserver
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/run", paramContractVersion),
		func(w http.ResponseWriter, req *http.Request) {
			model, err := r.model(req)
			if err != nil {
				r.sendError(w, err)
			}

			remoteCbh, err := newRemoteCallbackHandler(model.CallbackURL, r.debug)
			if err != nil {
				r.sendError(w, err)
			}

			if err := r.start(ctx, model, remoteCbh); err != nil {
				r.sendError(w, err)
			}
		}).
		Methods("PUT", "POST")

	srv := server.Webserver{
		Port:       r.serverOpts.port,
		SSLCrtFile: r.serverOpts.sslCrtFile,
		SSLKeyFile: r.serverOpts.sslKeyFile,
		Router:     router,
	}

	return srv.Start(ctx) //blocking until ctx gets closed
}

func (r *ComponentReconciler) start(ctx context.Context, model *Reconciliation, cbh CallbackHandler) error {
	//TODO: run in context with max 30min lifetime
	//TODO: assign to worker pool
	statusUpdater := newStatusUpdater(ctx, r.updateInterval, cbh, uint(r.maxRetries), r.debug)
	return (&runner{r}).Run(ctx, model, statusUpdater)
}

func (r *ComponentReconciler) sendError(w http.ResponseWriter, err error) {
	httpCode := 500
	http.Error(w, fmt.Sprintf("%s\n\n%s", http.StatusText(httpCode), err.Error()), httpCode)
}

func (r *ComponentReconciler) model(req *http.Request) (*Reconciliation, error) {
	params := server.NewParams(req)
	contactVersion, err := params.String(paramContractVersion)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		return nil, err
	}

	var model = r.modelForVersion(contactVersion)
	err = json.Unmarshal(b, model)
	if err != nil {
		return nil, err
	}

	return model, err
}

func (r *ComponentReconciler) modelForVersion(contactVersion string) *Reconciliation {
	return &Reconciliation{} //change this function if different contract versions have to be supported
}
