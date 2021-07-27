package compreconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"io/ioutil"
	"net/http"
	"strings"
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
	dependencies          []string
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

func (r *ComponentReconciler) validate() error {
	if r.serverConfig.port < 0 {
		return fmt.Errorf("server port cannot be < 0 (got %d)", r.serverConfig.port)
	}
	if r.serverConfig.port == 0 {
		r.serverConfig.port = defaultServerPort
	}
	if r.statusUpdaterConfig.interval < 0 {
		return fmt.Errorf("status updater interval cannot be < 0 (got %.1f secs)",
			r.statusUpdaterConfig.interval.Seconds())
	}
	if r.statusUpdaterConfig.interval == 0 {
		r.statusUpdaterConfig.interval = defaultInterval
	}
	if r.statusUpdaterConfig.retryDelay < 0 {
		return fmt.Errorf("status updater retry-delay cannot be < 0 (got %.1f secs)",
			r.statusUpdaterConfig.retryDelay.Seconds())
	}
	if r.statusUpdaterConfig.retryDelay == 0 {
		r.statusUpdaterConfig.retryDelay = defaultRetryDelay
	}
	if r.statusUpdaterConfig.maxRetries < 0 {
		return fmt.Errorf("status updater max-retries cannot be < 0 (got %d)",
			r.statusUpdaterConfig.maxRetries)
	}
	if r.statusUpdaterConfig.maxRetries == 0 {
		r.statusUpdaterConfig.maxRetries = defaultMaxRetries
	}
	if r.progressTrackerConfig.interval < 0 {
		return fmt.Errorf("progress tracker interval cannot be < 0 (got %.1f secs)",
			r.progressTrackerConfig.interval.Seconds())
	}
	if r.progressTrackerConfig.interval == 0 {
		r.progressTrackerConfig.interval = defaultInterval
	}
	if r.progressTrackerConfig.timeout < 0 {
		return fmt.Errorf("progress tracker timeout cannot be < 0 (got %.1f secs)",
			r.progressTrackerConfig.timeout.Seconds())
	}
	if r.progressTrackerConfig.timeout == 0 {
		r.progressTrackerConfig.timeout = defaultTimeout
	}
	if r.maxRetries < 0 {
		return fmt.Errorf("max-retries cannot be < 0 (got %d)", r.maxRetries)
	}
	if r.maxRetries == 0 {
		r.maxRetries = defaultMaxRetries
	}
	if r.retryDelay < 0 {
		return fmt.Errorf("retry-delay cannot be < 0 (got %.1f secs", r.retryDelay.Seconds())
	}
	if r.retryDelay == 0 {
		r.retryDelay = defaultRetryDelay
	}
	if r.workers < 0 {
		return fmt.Errorf("workers count cannot be < 0 (got %d)", r.workers)
	}
	if r.workers == 0 {
		r.workers = defaultWorkers
	}
	if r.timeout < 0 {
		return fmt.Errorf("timeout cannot be < 0 (got %.1f secs)", r.timeout.Seconds())
	}
	if r.timeout == 0 {
		r.timeout = defaultTimeout
	}
	return nil
}

func (r *ComponentReconciler) WithDependencies(components ...string) *ComponentReconciler {
	r.dependencies = components
	return r
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
	if err := r.validate(); err != nil {
		return err
	}

	localCbh, err := newLocalCallbackHandler(model.CallbackFct, r.debug)
	if err != nil {
		return err
	}

	runnerFct := r.newRunnerFct(ctx, model, localCbh)
	return runnerFct()
}

func (r *ComponentReconciler) StartRemote(ctx context.Context) error {
	if err := r.validate(); err != nil {
		return err
	}

	//start worker pool
	r.logger().Info(fmt.Sprintf("Starting worker pool with %d workers", r.workers))
	workerPool, err := ants.NewPool(r.workers, ants.WithNonblocking(true))
	if err != nil {
		return err
	}

	defer func() { //shutdown worker pool when stopping webserver
		r.logger().Info("Shutting down worker pool")
		workerPool.Release()
	}()

	//start webserver
	srv := server.Webserver{
		Logger:     r.logger(),
		Port:       r.serverConfig.port,
		SSLCrtFile: r.serverConfig.sslCrtFile,
		SSLKeyFile: r.serverConfig.sslKeyFile,
		Router:     r.newRouter(ctx, workerPool),
	}

	return srv.Start(ctx) //blocking until ctx gets closed
}

func (r *ComponentReconciler) newRouter(ctx context.Context, workerPool *ants.Pool) *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/run", paramContractVersion),
		func(w http.ResponseWriter, req *http.Request) {
			r.logger().Debug("Start processing request")
			model, err := r.model(req)
			if err != nil {
				r.logger().Warn(fmt.Sprintf("Unmarshalling of model failed: %s", err))
				r.sendResponse(w, http.StatusInternalServerError, err)
				return
			}
			r.logger().Debug(fmt.Sprintf("Model unmarshalled: %s", model))

			depMissing := r.dependenciesMissing(model)
			if len(depMissing) > 0 {
				r.logger().Debug(fmt.Sprintf("Found missing component dependencies: %s", strings.Join(depMissing, ", ")))
				r.sendResponse(w, http.StatusPreconditionRequired, HttpMissingDependenciesResponse{
					Dependencies: struct {
						Required []string
						Missing  []string
					}{Required: r.dependencies, Missing: depMissing},
				})
				return
			}
			//assign runner to worker
			remoteCbh, err := newRemoteCallbackHandler(model.CallbackURL, r.debug)
			if err != nil {
				r.logger().Warn(fmt.Sprintf("Could not create remote callback handler: %s", err))
				r.sendResponse(w, http.StatusInternalServerError, err)
				return
			}

			err = workerPool.Submit(func() {
				r.logger().Debug(fmt.Sprintf("Runner for model '%s' is assigned to worker", model))
				runnerFct := r.newRunnerFct(ctx, model, remoteCbh)
				if errRunner := runnerFct(); errRunner != nil {
					r.logger().Warn(
						fmt.Sprintf("Runner failed for model '%s': %v", model, errRunner))
					return
				}
			})
			if err != nil {
				r.logger().Warn(fmt.Sprintf("Runner for model '%s' could not be assigned to worker: %s", model, err))
				r.sendResponse(w, http.StatusInternalServerError, err)
				return
			}

			r.sendResponse(w, http.StatusOK, nil)
		}).
		Methods("PUT", "POST")
	return router
}

func (r *ComponentReconciler) dependenciesMissing(model *Reconciliation) []string {
	var missing []string
	for _, compDep := range r.dependencies {
		found := false
		for _, compReady := range model.ComponentsReady {
			if compReady == compDep { //check if required component is part of the components which are ready
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, compDep)
		}
	}
	return missing
}

func (r *ComponentReconciler) newRunnerFct(ctx context.Context, model *Reconciliation, callback CallbackHandler) func() error {
	r.logger().Debug(
		fmt.Sprintf("Creating new runner closure with execution timeout of %.1f secs", r.timeout.Seconds()))
	return func() error {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return (&runner{r}).Run(timeoutCtx, model, callback)
	}
}

func (r *ComponentReconciler) sendResponse(w http.ResponseWriter, httpCode int, response interface{}) {
	if err, ok := response.(error); ok { //convert to error response
		response = HttpErrorResponse{
			Error: err,
		}
	}
	w.WriteHeader(httpCode)
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		r.logger().Warn(fmt.Sprintf("Failed to encode response payload to JSON: %s", err))
		//send error response
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to encode response payload to JSON", http.StatusInternalServerError)
	}
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
