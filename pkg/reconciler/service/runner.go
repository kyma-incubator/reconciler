package service

import (
	"context"
	"fmt"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/heartbeat"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/adapter"
	"github.com/pkg/errors"
)

type runner struct {
	*ComponentReconciler
}

func (r *runner) Run(ctx context.Context, model *reconciler.Reconciliation, callback callback.Handler) error {
	heartbeatSender, err := heartbeat.NewHeartbeatSender(ctx, callback, r.logger, heartbeat.Config{
		Interval: r.heartbeatSenderConfig.interval,
		Timeout:  r.heartbeatSenderConfig.timeout,
	})
	if err != nil {
		return err
	}

	retryable := func(heartbeatSender *heartbeat.Sender) func() error {
		return func() error {
			if err := heartbeatSender.Running(); err != nil {
				r.logger.Warnf("Failed to start status updater: %s", err)
				return err
			}
			err := r.reconcile(ctx, model)
			if err != nil {
				r.logger.Warnf("Failing reconciliation of '%s' in version '%s' with profile '%s': %s",
					model.Component, model.Version, model.Profile, err)
				if heartbeatErr := heartbeatSender.Failed(err); heartbeatErr != nil {
					err = errors.Wrap(err, heartbeatErr.Error())
				}
			}
			return err
		}
	}(heartbeatSender)

	//retry the reconciliation in case of an error
	err = retry.Do(retryable,
		retry.Attempts(uint(r.maxRetries)),
		retry.Delay(r.retryDelay),
		retry.LastErrorOnly(false),
		retry.Context(ctx))

	if err == nil {
		r.logger.Infof("Reconciliation of component '%s' for version '%s' finished successfully",
			model.Component, model.Version)
		if err := heartbeatSender.Success(); err != nil {
			return err
		}
	} else {
		r.logger.Errorf("Retryable reconciliation of component '%s' for version '%s' failed consistently: giving up",
			model.Component, model.Version)
		if heartbeatErr := heartbeatSender.Error(err); heartbeatErr != nil {
			return errors.Wrap(err, heartbeatErr.Error())
		}
	}

	return err
}

func (r *runner) reconcile(ctx context.Context, model *reconciler.Reconciliation) error {
	kubeClient, err := adapter.NewKubernetesClient(model.Kubeconfig, r.logger, &adapter.Config{
		ProgressInterval: r.progressTrackerConfig.interval,
		ProgressTimeout:  r.progressTrackerConfig.timeout,
	})
	if err != nil {
		return err
	}

	chartProvider, err := r.newChartProvider()
	if err != nil {
		return errors.Wrap(err, "Failed to create chart provider instance")
	}

	wsFactory, err := r.workspaceFactory()
	if err != nil {
		return err
	}

	actionHelper := &ActionContext{
		KubeClient:       kubeClient,
		WorkspaceFactory: wsFactory,
		Context:          ctx,
		Logger:           r.logger,
		ChartProvider:    chartProvider,
	}

	if r.preReconcileAction != nil {
		if err := r.preReconcileAction.Run(model.Version, model.Profile, model.Configuration, actionHelper); err != nil {
			r.logger.Warnf("Pre-reconciliation action of '%s' with version '%s' failed: %s",
				model.Component, model.Version, err)
			return err
		}
	}

	if r.reconcileAction == nil {
		if err := r.install(ctx, chartProvider, model, kubeClient); err != nil {
			r.logger.Warnf("Default-reconciliation of '%s' with version '%s' failed: %s",
				model.Component, model.Version, err)
			return err
		}
	} else {
		if err := r.reconcileAction.Run(model.Version, model.Profile, model.Configuration, actionHelper); err != nil {
			r.logger.Warnf("Reconciliation action of '%s' with version '%s' failed: %s",
				model.Component, model.Version, err)
			return err
		}
	}

	if r.postReconcileAction != nil {
		if err := r.postReconcileAction.Run(model.Version, model.Profile, model.Configuration, actionHelper); err != nil {
			r.logger.Warnf("Post-reconciliation action of '%s' with version '%s' failed: %s",
				model.Component, model.Version, err)
			return err
		}
	}

	return nil
}

func (r *runner) install(ctx context.Context, chartProvider *chart.Provider, model *reconciler.Reconciliation, kubeClient kubernetes.Client) error {
	manifest, err := r.renderManifest(chartProvider, model)
	if err != nil {
		return err
	}

	resources, err := kubeClient.Deploy(ctx, manifest, model.Namespace, &LabelInterceptor{})

	if err == nil {
		r.logger.Debugf("Deployment of manifest finished successfully: %d resources deployed", len(resources))
	} else {
		r.logger.Warnf("Failed to deploy manifests on target cluster: %s", err)
	}

	return err
}

func (r *runner) renderManifest(chartProvider *chart.Provider, model *reconciler.Reconciliation) (string, error) {
	component := chart.NewComponentBuilder(model.Version, model.Component).
		WithProfile(model.Profile).
		WithNamespace(model.Namespace).
		WithConfiguration(model.Configuration).
		Build()

	var manifests []*chart.Manifest

	//get manifest of component
	chartManifest, err := chartProvider.RenderManifest(component)
	if err != nil {
		msg := fmt.Sprintf("Failed to get manifest for component '%s' in Kyma version '%s'",
			model.Component, model.Version)
		r.logger.Errorf("%s: %s", msg, err)
		return "", errors.Wrap(err, msg)
	}
	manifests = append(manifests, chartManifest)

	//get Kyma CRDs
	if model.InstallCRD {
		crdManifests, err := chartProvider.RenderCRD(model.Version)
		if err != nil {
			msg := fmt.Sprintf("Failed to get CRD manifests for Kyma version '%s'", model.Version)
			r.logger.Errorf("%s: %s", msg, err)
			return "", errors.Wrap(err, msg)
		}
		manifests = append(manifests, crdManifests...)
	}

	return chart.MergeManifests(manifests...), nil
}
