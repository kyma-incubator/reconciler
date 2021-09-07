package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/panjf2000/ants/v2"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gorilla/mux"
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
	defaultWorkspace     = "."
)

var (
	wsFactory *workspace.Factory //singleton
	m         sync.Mutex
)

type ActionContext struct {
	KubeClient       kubernetes.Client
	WorkspaceFactory *workspace.Factory
	Context          context.Context
	Logger           *zap.SugaredLogger
	ChartProvider    *chart.Provider
}

type Action interface {
	Run(version, profile string, configuration []reconciler.Configuration, helper *ActionContext) error
}

type ComponentReconciler struct {
	workspace             string
	dependencies          []string
	serverConfig          serverConfig
	heartbeatSenderConfig heartbeatSenderConfig
	progressTrackerConfig progressTrackerConfig
	//actions:
	preReconcileAction  Action
	reconcileAction     Action
	postReconcileAction Action
	//retry:
	maxRetries int
	retryDelay time.Duration
	//worker pool:
	timeout time.Duration
	workers int
	logger  *zap.SugaredLogger
	debug   bool
	mu      sync.Mutex
}

type heartbeatSenderConfig struct {
	interval time.Duration
	timeout  time.Duration
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

func NewComponentReconciler(reconcilerName string) (*ComponentReconciler, error) {
	log, err := logger.NewLogger(false)
	if err != nil {
		return nil, err
	}
	recon := &ComponentReconciler{
		workspace: defaultWorkspace,
		logger:    log,
	}
	RegisterReconciler(reconcilerName, recon) //add reconciler to registry
	return recon, nil
}

func UseGlobalWorkspaceFactory(workspaceFactory *workspace.Factory) error {
	if wsFactory != nil {
		return fmt.Errorf("workspace factory already defined: %s", wsFactory)
	}
	wsFactory = workspaceFactory
	return nil
}

func (r *ComponentReconciler) newChartProvider() (*chart.Provider, error) {
	wsFact, err := r.workspaceFactory()
	if err != nil {
		return nil, err
	}
	return chart.NewProvider(wsFact, r.logger)
}

func (r *ComponentReconciler) workspaceFactory() (*workspace.Factory, error) {
	m.Lock()
	defer m.Unlock()

	var err error
	if wsFactory == nil {
		r.logger.Debugf("Creating new workspace factory using storage directory '%s'", r.workspace)
		wsFactory, err = workspace.NewFactory(r.workspace, r.logger)
	}

	return wsFactory, err
}

func (r *ComponentReconciler) validate() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.serverConfig.port < 0 {
		return fmt.Errorf("server port cannot be < 0 (got %d)", r.serverConfig.port)
	}
	if r.serverConfig.port == 0 {
		r.serverConfig.port = defaultServerPort
	}
	if r.heartbeatSenderConfig.interval < 0 {
		return fmt.Errorf("heartbeat interval cannot be < 0 (got %.1f secs)",
			r.heartbeatSenderConfig.interval.Seconds())
	}
	if r.heartbeatSenderConfig.interval == 0 {
		r.heartbeatSenderConfig.interval = defaultInterval
	}
	if r.heartbeatSenderConfig.timeout < 0 {
		return fmt.Errorf("heartbeat sender timeouts cannot be < 0 (got %d)",
			r.heartbeatSenderConfig.timeout)
	}
	if r.heartbeatSenderConfig.timeout == 0 {
		r.heartbeatSenderConfig.timeout = defaultTimeout
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

func (r *ComponentReconciler) Debug() error {
	var err error
	r.logger, err = logger.NewLogger(true)
	r.debug = true
	return err
}

func (r *ComponentReconciler) WithWorkspace(workspace string) *ComponentReconciler {
	r.workspace = workspace
	return r
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

func (r *ComponentReconciler) WithPreReconcileAction(preReconcileAction Action) *ComponentReconciler {
	r.preReconcileAction = preReconcileAction
	return r
}

func (r *ComponentReconciler) WithReconcileAction(reconcileAction Action) *ComponentReconciler {
	r.reconcileAction = reconcileAction
	return r
}

func (r *ComponentReconciler) WithPostReconcileAction(postReconcileAction Action) *ComponentReconciler {
	r.postReconcileAction = postReconcileAction
	return r
}

func (r *ComponentReconciler) WithHeartbeatSenderConfig(interval, timeout time.Duration) *ComponentReconciler {
	r.heartbeatSenderConfig.interval = interval
	r.heartbeatSenderConfig.timeout = timeout
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

func (r *ComponentReconciler) StartLocal(ctx context.Context, model *reconciler.Reconciliation) error {
	//ensure model is valid
	if err := model.Validate(); err != nil {
		return err
	}
	//ensure reconciler is properly configured
	if err := r.validate(); err != nil {
		return err
	}

	localCbh, err := callback.NewLocalCallbackHandler(model.CallbackFunc, r.logger)
	if err != nil {
		return err
	}

	runnerFunc := r.newRunnerFunc(ctx, model, localCbh)
	return runnerFunc()
}

func (r *ComponentReconciler) StartRemote(ctx context.Context) error {
	if err := r.validate(); err != nil {
		return err
	}

	//start worker pool
	r.logger.Infof("Starting worker pool with %d workers", r.workers)
	workerPool, err := ants.NewPool(r.workers, ants.WithNonblocking(true))
	if err != nil {
		return err
	}

	defer func() { //shutdown worker pool when stopping webserver
		r.logger.Info("Shutting down worker pool")
		workerPool.Release()
	}()

	//start webserver
	srv := server.Webserver{
		Logger:     r.logger,
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
			r.logger.Debug("Start processing request")

			//marshal model
			model, err := r.model(req)
			if err != nil {
				r.logger.Warnf("Unmarshalling of model failed: %s", err)
				r.sendResponse(w, http.StatusInternalServerError, err)
				return
			}
			r.logger.Debugf("Model unmarshalled: %s", model)

			//validate model
			if err := model.Validate(); err != nil {
				r.sendResponse(w, http.StatusBadRequest, err)
				return
			}

			//check whether all dependencies are fulfilled
			depMissing := r.dependenciesMissing(model)
			if len(depMissing) > 0 {
				r.logger.Debugf("Found missing component dependencies: %s", strings.Join(depMissing, ", "))
				r.sendResponse(w, http.StatusPreconditionRequired, reconciler.HTTPMissingDependenciesResponse{
					Dependencies: struct {
						Required []string
						Missing  []string
					}{Required: r.dependencies, Missing: depMissing},
				})
				return
			}

			//enrich logger with correlation ID and component name
			loggerNew, err := logger.NewLogger(r.debug)
			if err != nil {
				r.logger.Errorf("Could not create a new logger that is correlationID-aware: %s", err)
				return
			}
			r.logger = loggerNew.With(zap.Field{Key: "correlation-id", Type: zapcore.StringType, String: model.CorrelationID}, zap.Field{Key: "component-name", Type: zapcore.StringType, String: model.Component})

			//create callback handler
			remoteCbh, err := callback.NewRemoteCallbackHandler(model.CallbackURL, r.logger)
			if err != nil {
				r.logger.Warnf("Could not create remote callback handler: %s", err)
				r.sendResponse(w, http.StatusInternalServerError, err)
				return
			}

			//assign runner to worker
			err = workerPool.Submit(func() {
				r.logger.Debugf("Runner for model '%s' is assigned to worker", model)
				runnerFunc := r.newRunnerFunc(ctx, model, remoteCbh)
				if errRunner := runnerFunc(); errRunner != nil {
					r.logger.Warnf("Runner failed for model '%s': %v", model, errRunner)
					return
				}
			})

			//check if execution of worker was successful
			if err != nil {
				r.logger.Warnf("Runner for model '%s' could not be assigned to worker: %s", model, err)
				r.sendResponse(w, http.StatusInternalServerError, err)
				return
			}

			//done
			r.logger.Debug("Request successfully processed")
			r.sendResponse(w, http.StatusOK, nil)
		}).
		Methods("PUT", "POST")
	return router
}

func (r *ComponentReconciler) dependenciesMissing(model *reconciler.Reconciliation) []string {
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

func (r *ComponentReconciler) newRunnerFunc(ctx context.Context, model *reconciler.Reconciliation, callback callback.Handler) func() error {
	r.logger.Debugf("Creating new runner closure with execution timeout of %.1f secs", r.timeout.Seconds())
	return func() error {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return (&runner{r}).Run(timeoutCtx, model, callback)
	}
}

func (r *ComponentReconciler) sendResponse(w http.ResponseWriter, httpCode int, response interface{}) {
	if err, ok := response.(error); ok { //convert to error response
		response = reconciler.HTTPErrorResponse{
			Error: err.Error(),
		}
	}
	w.WriteHeader(httpCode)
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		r.logger.Warnf("Failed to encode response payload to JSON: %s", err)
		//send error response
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to encode response payload to JSON", http.StatusInternalServerError)
	}
}

func (r *ComponentReconciler) model(req *http.Request) (*reconciler.Reconciliation, error) {
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

func (r *ComponentReconciler) modelForVersion(contractVersion string) (*reconciler.Reconciliation, error) {
	if contractVersion == "" {
		return nil, fmt.Errorf("contract version cannot be empty")
	}
	return &reconciler.Reconciliation{}, nil //change this function if different contract versions have to be supported
}
