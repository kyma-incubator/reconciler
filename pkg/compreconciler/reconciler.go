package compreconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"

	"go.uber.org/zap"
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
	defaultRetryDelay    = 30 * time.Second
	defaultTimeout       = 10 * time.Minute
	defaultWorkers       = 100
)

type Action interface {
	Run(version string, kubeClient *kubernetes.Clientset) error
}

type ComponentReconciler struct {
	debug                 bool
	serverConfig          serverConfig
	statusUpdaterConfig   statusUpdaterConfig
	progressTrackerConfig progressTrackerConfig
	chartProvider         *chart.Provider
	//actions:
	preInstallAction  Action
	installAction     Action
	postInstallAction Action
	//retry:
	maxRetries int
	retryDelay time.Duration
	//worker pool:
	timeout time.Duration
	workers int
}

type statusUpdaterConfig struct {
	interval   time.Duration
	maxRetries int
	retryDelay time.Duration
}

type progressTrackerConfig struct {
	interval time.Duration
	timeout  time.Duration
}

type serverConfig struct {
	port       int
	sslCrtFile string
	sslKeyFile string
}

func NewComponentReconciler(chartProvider *chart.Provider) *ComponentReconciler {
	return &ComponentReconciler{
		chartProvider: chartProvider,
	}
}

func (r *ComponentReconciler) logger() *zap.Logger {
	return logger.NewOptionalLogger(r.debug)
}

func (r *ComponentReconciler) validate() {
	if r.serverConfig.port <= 0 {
		r.serverConfig.port = defaultServerPort
	}
	if r.statusUpdaterConfig.interval <= 0 {
		r.statusUpdaterConfig.interval = defaultInterval
	}
	if r.statusUpdaterConfig.retryDelay <= 0 {
		r.statusUpdaterConfig.retryDelay = defaultRetryDelay
	}
	if r.statusUpdaterConfig.maxRetries <= 0 {
		r.statusUpdaterConfig.maxRetries = defaultMaxRetries
	}
	if r.progressTrackerConfig.interval <= 0 {
		r.progressTrackerConfig.interval = defaultInterval
	}
	if r.progressTrackerConfig.timeout <= 0 {
		r.progressTrackerConfig.timeout = defaultTimeout
	}
	if r.maxRetries <= 0 {
		r.maxRetries = defaultMaxRetries
	}
	if r.retryDelay <= 0 {
		r.retryDelay = defaultRetryDelay
	}
	if r.workers <= 0 {
		r.workers = defaultWorkers
	}
	if r.timeout <= 0 {
		r.timeout = defaultTimeout
	}
}

func (r *ComponentReconciler) WithRetry(maxRetries int, retryDelay time.Duration) *ComponentReconciler {
	r.maxRetries = maxRetries
	r.retryDelay = retryDelay
	return r
}

func (r *ComponentReconciler) WithWorkers(workers int, timeout time.Duration) *ComponentReconciler {
	r.workers = workers
	r.timeout = timeout
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

func (r *ComponentReconciler) WithStatusUpdaterConfig(interval time.Duration, maxRetries int, retryDelay time.Duration) *ComponentReconciler {
	r.statusUpdaterConfig.interval = interval
	r.statusUpdaterConfig.maxRetries = maxRetries
	r.statusUpdaterConfig.retryDelay = retryDelay
	return r
}

func (r *ComponentReconciler) WithServerConfig(port int, sslCrtFile, sslKeyFile string) *ComponentReconciler {
	r.serverConfig.port = port
	r.serverConfig.sslCrtFile = sslCrtFile
	r.serverConfig.sslKeyFile = sslKeyFile
	return r
}

func (r *ComponentReconciler) WithProgressTrackerConfig(interval, timeout time.Duration) *ComponentReconciler {
	r.progressTrackerConfig.interval = interval
	r.progressTrackerConfig.timeout = timeout
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

	runnerFct := r.newRunnerFct(ctx, model, localCbh)
	return runnerFct()
}

func (r *ComponentReconciler) StartRemote(ctx context.Context) error {
	r.validate()

	//start worker pool
	r.logger().Debug(fmt.Sprintf("Starting worker pool with %d workers", r.workers))
	workerPool, err := ants.NewPool(r.workers, ants.WithNonblocking(true))
	if err != nil {
		return err
	}
	defer workerPool.Release()

	//create webserver
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/run", paramContractVersion),
		func(w http.ResponseWriter, req *http.Request) {
			model, err := r.model(req)
			if err != nil {
				r.sendError(w, err)
				return
			}

			//assign runner to worker
			remoteCbh, err := newRemoteCallbackHandler(model.CallbackURL, r.debug)
			if err != nil {
				r.sendError(w, err)
				return
			}

			err = workerPool.Submit(func() {
				runnerFct := r.newRunnerFct(ctx, model, remoteCbh)
				if errRunner := runnerFct(); err != nil {
					r.sendError(w, errRunner)
					return
				}
			})
			if err != nil {
				r.sendError(w, err)
				return
			}
		}).
		Methods("PUT", "POST")

	//start webserver
	r.logger().Debug(fmt.Sprintf("Starting webserver on port %d", r.serverConfig.port))
	srv := server.Webserver{
		Port:       r.serverConfig.port,
		SSLCrtFile: r.serverConfig.sslCrtFile,
		SSLKeyFile: r.serverConfig.sslKeyFile,
		Router:     router,
	}

	return srv.Start(ctx) //blocking until ctx gets closed
}

func (r *ComponentReconciler) newRunnerFct(ctx context.Context, model *Reconciliation, callback CallbackHandler) func() error {
	return func() error {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return (&runner{r}).Run(timeoutCtx, model, callback)
	}
}

func (r *ComponentReconciler) sendError(w http.ResponseWriter, err error) {
	httpCode := 500
	http.Error(w, fmt.Sprintf("%s\n\n%s", http.StatusText(httpCode), err.Error()), httpCode)
}

func (r *ComponentReconciler) model(req *http.Request) (*Reconciliation, error) {
	params := server.NewParams(req)
	contractVersion, err := params.String(paramContractVersion)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	model, err := r.modelForVersion(contractVersion)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, model)
	if err != nil {
		return nil, err
	}

	return model, err
}

func (r *ComponentReconciler) modelForVersion(contractVersion string) (*Reconciliation, error) {
	if contractVersion == "" {
		return nil, fmt.Errorf("contract version cannot be empty")
	}
	return &Reconciliation{}, nil //change this function if different contract versions have to be supported
}
