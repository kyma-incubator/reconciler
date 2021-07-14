package compreconciler

import (
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/server"
)

const (
	paramContractVersion = "version"
	defaultServerPort    = 5
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
	//chartProvider     *chart.Provider
	maxRetries int
}

type serverOpts struct {
	port       int
	sslCrtFile string
	sslKeyFile string
}

func NewComponentReconciler() *ComponentReconciler {
	return &ComponentReconciler{}
}

func (r *ComponentReconciler) validate() {
	if r.interval <= 0 {
		r.interval = defaultInterval
	}
	if r.maxRetries <= 0 {
		r.maxRetries = defaultMaxRetries
	}
}

func (r *ComponentReconciler) Configure(interval time.Duration, maxRetries int) *ComponentReconciler {
	r.interval = interval
	r.maxRetries = maxRetries
	if r.serverOpts.port <= 0 {
		r.serverOpts.port = defaultServerPort
	}
	return r
}

func (r *ComponentReconciler) WithServerConfiguration(port int, sslCrtFile, sslKeyFile string) *ComponentReconciler {
	r.serverOpts.port = port
	r.serverOpts.sslCrtFile = sslCrtFile
	r.serverOpts.sslKeyFile = sslKeyFile
	return r
}

func (r *ComponentReconciler) WithPreInstallAction(port int, sslCrtFile, sslKeyFile string) *ComponentReconciler {
	r.serverOpts.port = port
	r.serverOpts.sslCrtFile = sslCrtFile
	r.serverOpts.sslKeyFile = sslKeyFile
	return r
}

func (r *ComponentReconciler) WithInstallAction(port int, sslCrtFile, sslKeyFile string) *ComponentReconciler {
	r.serverOpts.port = port
	return r
}

func (r *ComponentReconciler) WithPostInstallAction(port int, sslCrtFile, sslKeyFile string) *ComponentReconciler {
	r.serverOpts.port = port
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
			err := (&runner{
				preInstallAction:  r.preInstallAction,
				installAction:     r.installAction,
				postInstallAction: r.postInstallAction,
				maxRetries:        r.maxRetries,
				interval:          r.interval,
				debug:             r.debug,
			}).Run(w, req)
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

	return server.Start()
}

func sendError(w http.ResponseWriter, httpCode int, err error) {
	http.Error(w, fmt.Sprintf("%s\n\n%s", http.StatusText(httpCode), err.Error()), httpCode)
}
