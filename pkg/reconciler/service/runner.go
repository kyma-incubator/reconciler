package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"go.uber.org/zap"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/heartbeat"
	"github.com/pkg/errors"
)

type runner struct {
	*ComponentReconciler
	install *Install
	logger  *zap.SugaredLogger
}

func (r *runner) Run(ctx context.Context, task *reconciler.Task, callback callback.Handler) error {
	heartbeatSender, err := heartbeat.NewHeartbeatSender(ctx, callback, r.logger, heartbeat.Config{
		Interval: r.heartbeatSenderConfig.interval,
		Timeout:  r.heartbeatSenderConfig.timeout,
	})
	if err != nil {
		return err
	}
	var retryID string
	retryable := func() error {
		retryID = uuid.NewString()
		if err := heartbeatSender.Running(retryID); err != nil {
			r.logger.Warnf("Runner: failed to start status updater: %s", err)
			return err
		}
		err := r.reconcile(ctx, task)
		if err != nil {
			r.logger.Warnf("Runner: failing reconciliation of '%s' in version '%s' with profile '%s': %s",
				task.Component, task.Version, task.Profile, err)
			if heartbeatErr := heartbeatSender.Failed(err, retryID); heartbeatErr != nil {
				err = errors.Wrap(err, heartbeatErr.Error())
			}
		}
		return err
	}

	startTime := time.Now()
	//retry the reconciliation in case of an error
	err = retry.Do(retryable,
		retry.Attempts(uint(task.ComponentConfiguration.MaxRetries)),
		retry.Delay(r.retryDelay),
		retry.LastErrorOnly(false),
		retry.Context(ctx))

	processingDuration := time.Now().Sub(startTime)
	if err == nil {
		r.logger.Infof("Runner: reconciliation of component '%s' for version '%s' finished successfully",
			task.Component, task.Version)
		if err := heartbeatSender.Success(retryID); err != nil {
			return err
		}
		err := r.updateProcessingDurationOccupancy(processingDuration.Milliseconds(), task)
		if err != nil {
			r.logger.Warnf("Could not update the processingDuration: %s", err.Error())
		}
	} else if ctx.Err() != nil {
		r.logger.Infof("Runner: reconciliation of component '%s' for version '%s' terminated because context was closed",
			task.Component, task.Version)
		return err
	} else {
		r.logger.Errorf("Runner: retryable reconciliation of component '%s' for version '%s' failed consistently: giving up",
			task.Component, task.Version)
		if heartbeatErr := heartbeatSender.Error(err, retryID); heartbeatErr != nil {
			return errors.Wrap(err, heartbeatErr.Error())
		}
		err := r.updateProcessingDurationOccupancy(processingDuration.Milliseconds(), task)
		if err != nil {
			r.logger.Warnf("Could not update the processingDuration: %s", err.Error())
		}
	}

	return err
}

func (r *runner) reconcile(ctx context.Context, task *reconciler.Task) error {
	kubeClient, err := k8s.NewKubernetesClient(task.Kubeconfig, r.logger, &k8s.Config{
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
			r.logger.Debugf("Runner: Pre-%s action of '%s' with version '%s' failed: %s",
				task.Type, task.Component, task.Version, err)
			return err
		}
	}

	if act == nil {
		if err := r.install.Invoke(ctx, chartProvider, task, kubeClient); err != nil {
			r.logger.Debugf("Runner: Default-%s action of '%s' with version '%s' failed: %s",
				task.Type, task.Component, task.Version, err)
			return err
		}
	} else {
		if err := act.Run(actionHelper); err != nil {
			r.logger.Debugf("Runner: %s action of '%s' with version '%s' failed: %s",
				strings.Title(string(task.Type)), task.Component, task.Version, err)
			return err
		}
	}

	if post != nil {
		if err := post.Run(actionHelper); err != nil {
			r.logger.Debugf("Runner: Post-%s action of '%s' with version '%s' failed: %s",
				task.Type, task.Component, task.Version, err)
			return err
		}
	}

	return nil
}

func processingDurationCallbackURL(callbackURL string) (string, error) {

	if callbackURL == "" {
		return "", fmt.Errorf("error parsing callback URL from model: received empty string")
	}
	processingDurationURLTemplate := "%s/%s"
	return fmt.Sprintf(processingDurationURLTemplate, callbackURL, "processingDuration"), nil
}

func (r *runner) updateProcessingDurationOccupancy(processingDuration int64, task *reconciler.Task) error {
	processingDurationCallbackURL, err := processingDurationCallbackURL(task.CallbackURL)
	if err != nil {
		return fmt.Errorf("failed to parse callbackURL '%s': %s", task.CallbackURL, err)
	}

	httpProcessingDurationUpdateRequest := reconciler.ProcessingDuration{
		Duration: int(processingDuration),
	}
	jsonPayload, err := json.Marshal(httpProcessingDurationUpdateRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal HTTP payload to update processing duration of operation with correlationID '%s': %s", task.CorrelationID, err)
	}
	resp, err := http.Post(processingDurationCallbackURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode <= 299 {
		r.logger.Infof("ProcessingDuration for operation with correlationID '%s' updated occupancy successfully", task.CorrelationID)
		return nil
	}

	return fmt.Errorf("mothership failed to update processingDUration for operation with correlationID '%s' with status code: '%d'", task.CorrelationID, resp.StatusCode)
}
