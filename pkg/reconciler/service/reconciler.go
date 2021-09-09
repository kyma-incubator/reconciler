package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"go.uber.org/zap"
)

const (
	defaultServerPort = 8080
	defaultMaxRetries = 5
	defaultInterval   = 30 * time.Second
	defaultRetryDelay = 30 * time.Second
	defaultTimeout    = 10 * time.Minute
	defaultWorkers    = 100
	defaultWorkspace  = "."
)

var (
	wsFactory *workspace.Factory //singleton
	m         sync.Mutex
)

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

func (r *ComponentReconciler) StartRemote(ctx context.Context) (*WorkerPool, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	return newWorkerPool(ctx, r)
}

func (r *ComponentReconciler) newRunnerFunc(ctx context.Context, model *reconciler.Reconciliation, callback callback.Handler) func() error {
	r.logger.Debugf("Creating new runner closure with execution timeout of %.1f secs", r.timeout.Seconds())
	return func() error {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return (&runner{r}).Run(timeoutCtx, model, callback)
	}
}
