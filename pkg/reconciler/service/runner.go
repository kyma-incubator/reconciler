package service

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/heartbeat"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/adapter"
	"github.com/pkg/errors"
)

type runner struct {
	*ComponentReconciler
	install *Install

	logger *zap.SugaredLogger
}

func (r *runner) Run(ctx context.Context, task *reconciler.Task, callback callback.Handler) error {
	heartbeatSender, err := heartbeat.NewHeartbeatSender(ctx, callback, r.logger, heartbeat.Config{
		Interval: r.heartbeatSenderConfig.interval,
		Timeout:  r.heartbeatSenderConfig.timeout,
	})
	if err != nil {
		return err
	}

	retryable := func() error {
		if err := heartbeatSender.Running(); err != nil {
			r.logger.Warnf("Failed to start status updater: %s", err)
			return err
		}
		err := r.reconcile(ctx, task)
		if err != nil {
			r.logger.Warnf("Failing reconciliation of '%s' in version '%s' with profile '%s': %s",
				task.Component, task.Version, task.Profile, err)
			if heartbeatErr := heartbeatSender.Failed(err); heartbeatErr != nil {
				err = errors.Wrap(err, heartbeatErr.Error())
			}
		}
		return err
	}

	//retry the reconciliation in case of an error
	err = retry.Do(retryable,
		retry.Attempts(uint(r.maxRetries)),
		retry.Delay(r.retryDelay),
		retry.LastErrorOnly(false),
		retry.Context(ctx))

	if err == nil {
		r.logger.Infof("Reconciliation of component '%s' for version '%s' finished successfully",
			task.Component, task.Version)
		if err := heartbeatSender.Success(); err != nil {
			return err
		}
	} else if ctx.Err() != nil {
		r.logger.Infof("Reconciliation of component '%s' for version '%s' terminated because context was closed",
			task.Component, task.Version)
		return err
	} else {
		r.logger.Errorf("Retryable reconciliation of component '%s' for version '%s' failed consistently: giving up",
			task.Component, task.Version)
		if heartbeatErr := heartbeatSender.Error(err); heartbeatErr != nil {
			return errors.Wrap(err, heartbeatErr.Error())
		}
	}

	return err
}

func (r *runner) reconcile(ctx context.Context, task *reconciler.Task) error {
	kubeClient, err := adapter.NewKubernetesClient(task.Kubeconfig, r.logger, &adapter.Config{
		ProgressInterval: r.progressTrackerConfig.interval,
		ProgressTimeout:  r.progressTrackerConfig.timeout,
	})
	if err != nil {
		return err
	}

	chartProvider, err := r.newChartProvider(task.Repository)
	if err != nil {
		return errors.Wrap(err, "Failed to create chart provider instance")
	}

	wsFactory, err := r.workspaceFactory(task.Repository)
	if err != nil {
		return err
	}

	actionHelper := &ActionContext{
		KubeClient:       kubeClient,
		WorkspaceFactory: *wsFactory,
		Context:          ctx,
		Logger:           r.logger,
		ChartProvider:    chartProvider,
		Task:             task,
	}

	// Identify the right action set to use (reconcile/delete)
	pre, act, post := r.preReconcileAction, r.reconcileAction, r.postReconcileAction
	if task.Type == model.OperationTypeDelete {
		pre, act, post = r.preDeleteAction, r.deleteAction, r.postDeleteAction
	}

	if pre != nil {
		if err := pre.Run(actionHelper); err != nil {
			r.logger.Warnf("Pre-%s action of '%s' with version '%s' failed: %s",
				task.Type, task.Component, task.Version, err)
			return err
		}
	}

	if act == nil {
		if err := r.install.Invoke(ctx, chartProvider, task, kubeClient); err != nil {
			r.logger.Warnf("Default-%s action of '%s' with version '%s' failed: %s",
				task.Type, task.Component, task.Version, err)
			return err
		}
	} else {
		if err := act.Run(actionHelper); err != nil {
			r.logger.Warnf("%s action of '%s' with version '%s' failed: %s",
				strings.Title(string(task.Type)), task.Component, task.Version, err)
			return err
		}
	}

	if post != nil {
		if err := r.postReconcileAction.Run(actionHelper); err != nil {
			r.logger.Warnf("Post-%s action of '%s' with version '%s' failed: %s",
				task.Type, task.Component, task.Version, err)
			return err
		}
	}

	return nil
}
