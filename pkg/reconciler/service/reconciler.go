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
	defaultMaxRetries = 5
	defaultInterval   = 30 * time.Second
	defaultRetryDelay = 30 * time.Second
	defaultTimeout    = 10 * time.Minute
	defaultWorkers    = 100
	defaultWorkspace  = "."
)

var (
	wsFactory workspace.Factory //singleton
	m         sync.Mutex
)

type ComponentReconciler struct {
	workspace             string
	dependencies          []string
	heartbeatSenderConfig heartbeatSenderConfig
	progressTrackerConfig progressTrackerConfig
	//reconcile actions:
	preReconcileAction  Action
	reconcileAction     Action
	postReconcileAction Action
	//delete actions:
	preDeleteAction  Action
	deleteAction     Action
	postDeleteAction Action
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

func NewComponentReconciler(reconcilerName string) (*ComponentReconciler, error) {
	recon := &ComponentReconciler{
		workspace: defaultWorkspace,
		logger:    logger.NewLogger(false),
	}

	RegisterReconciler(reconcilerName, recon) //add reconciler to registry

	return recon, nil
}

func UseGlobalWorkspaceFactory(workspaceFactory workspace.Factory) error {
	if wsFactory != nil {
		return fmt.Errorf("workspace factory already defined: %s", wsFactory)
	}
	wsFactory = workspaceFactory
	return nil
}

func RefreshGlobalWorkspaceFactory(workspaceFactory workspace.Factory) error {
	wsFactory = workspaceFactory
	return nil
}

func (r *ComponentReconciler) newChartProvider(repo *reconciler.Repository) (*chart.DefaultProvider, error) {
	wsFact, err := r.workspaceFactory(repo)
	if err != nil {
		return nil, err
	}
	return chart.NewDefaultProvider(*wsFact, r.logger)
}

func (r *ComponentReconciler) workspaceFactory(repo *reconciler.Repository) (*workspace.Factory, error) {
	m.Lock()
	defer m.Unlock()

	var err error
	if wsFactory == nil {
		r.logger.Debugf("Creating new workspace factory using storage directory '%s'", r.workspace)
		wsFactory, err = workspace.NewFactory(repo, r.workspace, r.logger)
	}

	return &wsFactory, err
}

func (r *ComponentReconciler) validate() error {
	r.mu.Lock()
	defer r.mu.Unlock()

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

func (r *ComponentReconciler) Debug() *ComponentReconciler {
	r.logger = logger.NewLogger(true)
	r.debug = true
	return r
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

func (r *ComponentReconciler) WithPreDeleteAction(preDeleteAction Action) *ComponentReconciler {
	r.preDeleteAction = preDeleteAction
	return r
}

func (r *ComponentReconciler) WithDeleteAction(deleteAction Action) *ComponentReconciler {
	r.deleteAction = deleteAction
	return r
}

func (r *ComponentReconciler) WithPostDeleteAction(postDeleteAction Action) *ComponentReconciler {
	r.postDeleteAction = postDeleteAction
	return r
}

func (r *ComponentReconciler) WithHeartbeatSenderConfig(interval, timeout time.Duration) *ComponentReconciler {
	r.heartbeatSenderConfig.interval = interval
	r.heartbeatSenderConfig.timeout = timeout
	return r
}

func (r *ComponentReconciler) WithProgressTrackerConfig(interval, timeout time.Duration) *ComponentReconciler {
	r.progressTrackerConfig.interval = interval
	r.progressTrackerConfig.timeout = timeout
	return r
}

func (r *ComponentReconciler) StartLocal(ctx context.Context, model *reconciler.Task, logger *zap.SugaredLogger) error {
	//ensure model is valid
	if err := model.Validate(); err != nil {
		return err
	}
	//ensure reconciler is properly configured
	if err := r.validate(); err != nil {
		return err
	}

	localCbh, err := callback.NewLocalCallbackHandler(model.CallbackFunc, logger)
	if err != nil {
		return err
	}

	runnerFunc := r.newRunnerFunc(ctx, model, localCbh, logger)
	return runnerFunc()
}

func (r *ComponentReconciler) StartRemote(ctx context.Context) (*WorkerPool, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	return newWorkerPoolBuilder(&dependencyChecker{r.dependencies}, r.newRunnerFunc).
		WithPoolSize(r.workers).
		WithDebug(r.debug).
		Build(ctx)
}

func (r *ComponentReconciler) newRunnerFunc(ctx context.Context, model *reconciler.Task, callback callback.Handler, logger *zap.SugaredLogger) func() error {
	r.logger.Debugf("Creating new runner closure with execution timeout of %.1f secs", r.timeout.Seconds())
	return func() error {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return (&runner{r, NewInstall(logger), logger}).Run(timeoutCtx, model, callback)
	}
}
