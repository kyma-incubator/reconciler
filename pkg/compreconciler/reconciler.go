package compreconciler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/chart"
	"github.com/kyma-incubator/reconciler/pkg/server"
)

const (
	paramContractVersion = "version"
	defaultServerPort    = 8080
	defaultMaxRetries    = 5
	defaultInterval      = 30 * time.Second
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
	interval          time.Duration
	chartProvider     *chart.Provider
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
	if r.interval <= 0 {
		r.interval = defaultInterval
	}
	if r.maxRetries <= 0 {
		r.maxRetries = defaultMaxRetries
	}
	if r.serverOpts.port <= 0 {
		r.serverOpts.port = defaultServerPort
	}
}

func (r *ComponentReconciler) Configure(interval time.Duration, maxRetries int) *ComponentReconciler {
	r.interval = interval
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

func (r *ComponentReconciler) Start() error {
	r.validate()

	//create webserver
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/run", paramContractVersion),
		func(w http.ResponseWriter, req *http.Request) {
			err := (&runner{r}).Run(w, req)
			if err != nil {
				sendError(w, 500, err)
			}
		}).
		Methods("PUT", "POST")

	server := server.Webserver{
		Port:       r.serverOpts.port,
		SSLCrtFile: r.serverOpts.sslCrtFile,
		SSLKeyFile: r.serverOpts.sslKeyFile,
		Router:     router,
	}

	return server.Start(context.Background())
}

func sendError(w http.ResponseWriter, httpCode int, err error) {
	http.Error(w, fmt.Sprintf("%s\n\n%s", http.StatusText(httpCode), err.Error()), httpCode)
}
