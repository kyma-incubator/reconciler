package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/progress"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/status"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type runner struct {
	*ComponentReconciler
}

func (r *runner) Run(ctx context.Context, model *reconciler.Reconciliation, callback callback.Handler) error {
	statusUpdater, err := status.NewStatusUpdater(ctx, callback, r.logger, status.Config{
		Interval: r.statusUpdaterConfig.interval,
		Timeout:  r.statusUpdaterConfig.timeout,
	})
	if err != nil {
		return err
	}

	retryable := func(statusUpdater *status.Updater) func() error {
		return func() error {
			if err := statusUpdater.Running(); err != nil {
				r.logger.Warnf("Failed to start status updater: %s", err)
				return err
			}
			err := r.reconcile(ctx, model)
			if err == nil {
				r.logger.Infof("Reconciliation successful of '%s' in version '%s' with profile '%s'",
					model.Component, model.Version, model.Profile)
			} else {
				r.logger.Warnf("Reconciliation of '%s' in version '%s' with profile '%s': %s",
					model.Component, model.Version, model.Profile, err)
				if errUpdater := statusUpdater.Failed(); errUpdater != nil {
					err = errors.Wrap(err, errUpdater.Error())
				}
			}
			return err
		}
	}(statusUpdater)

	//retry the reconciliation in case of an error
	err = retry.Do(retryable,
		retry.Attempts(uint(r.maxRetries)),
		retry.Delay(r.retryDelay),
		retry.LastErrorOnly(false),
		retry.Context(ctx))

	if err == nil {
		r.logger.Infof("Reconciliation of component '%s' for version '%s' finished successfully",
			model.Component, model.Version)
		if err := statusUpdater.Success(); err != nil {
			return err
		}
	} else {
		r.logger.Warnf("Retryable reconciliation of component '%s' for version '%s' failed consistently: giving up",
			model.Component, model.Version)
		if err := statusUpdater.Error(); err != nil {
			return err
		}
	}

	return err
}

func (r *runner) reconcile(ctx context.Context, model *reconciler.Reconciliation) error {
	kubeClient, err := kubernetes.NewKubernetesClient(model.Kubeconfig, r.logger)
	if err != nil {
		return err
	}

	actionHelper := &ActionContext{
		KubeClient:       kubeClient,
		WorkspaceFactory: r.workspaceFactory(),
		Context:          ctx,
		Logger:           r.logger,
	}

	if r.preReconcileAction != nil {
		if err := r.preReconcileAction.Run(model.Version, model.Profile, model.Configuration, actionHelper); err != nil {
			r.logger.Warnf("Pre-reconciliation action of '%s' with version '%s' failed: %s",
				model.Component, model.Version, err)
			return err
		}
	}

	if r.reconcileAction == nil {
		if err := r.install(ctx, model, kubeClient); err != nil {
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

func (r *runner) install(ctx context.Context, model *reconciler.Reconciliation, kubeClient kubernetes.Client) error {
	manifest, err := r.renderManifest(model)
	if err != nil {
		return err
	}

	resources, err := kubeClient.Deploy(manifest, &LabelInterceptor{})

	if err != nil {
		r.logger.Warnf("Failed to deploy manifests on target cluster: %s", err)
		return err
	}

	return r.trackProgress(ctx, kubeClient, resources) //blocking call
}

func (r *runner) renderManifest(model *reconciler.Reconciliation) (string, error) {
	chartProvider, err := r.newChartProvider()
	if err != nil {
		return "", errors.Wrap(err, "Failed to create chart provider instance")
	}
	manifests, err := chartProvider.Manifests(r.newComponentSet(model), model.InstallCRD, &chart.Options{})
	if err != nil {
		msg := fmt.Sprintf("Failed to render manifest for component '%s'", model.Component)
		r.logger.Warn(msg)
		return "", errors.Wrap(err, msg)
	}

	var buffer bytes.Buffer
	r.logger.Debugf("Rendering of component '%s' returned %d manifests", model.Component, len(manifests))
	for _, manifest := range manifests {
		if !model.InstallCRD && manifest.Type == components.CRD {
			r.logger.Errorf("Illegal state detected! "+
				"No CRDs were requested but chartProvider returned CRD manifest: '%s'", manifest.Name)
		}
		buffer.WriteString("---\n")
		buffer.WriteString(fmt.Sprintf("# Manifest of %s '%s'\n", manifest.Type, model.Component))
		buffer.WriteString(manifest.Manifest)
		buffer.WriteString("\n")
	}
	return buffer.String(), nil
}

func (r *runner) trackProgress(ctx context.Context, kubeClient kubernetes.Client, resources []*kubernetes.Resource) error {
	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}
	//get resources defined in manifest
	pt, err := progress.NewProgressTracker(ctx, clientSet, r.logger, progress.Config{
		Timeout:  r.progressTrackerConfig.timeout,
		Interval: r.progressTrackerConfig.interval,
	})
	if err != nil {
		return err
	}
	//watch progress of installed resources
	for _, resource := range resources {
		watchable, err := progress.NewWatchableResource(resource.Kind) //convert "kind" to watchable
		if err != nil {
			r.logger.Debugf("Ignoring non-watchable resource: %s", resource)
			continue //not watchable resource: ignore it
		}
		pt.AddResource(
			watchable,
			resource.Namespace,
			resource.Name,
		)
	}
	r.logger.Debug("Start watching installation progress")
	return pt.Watch() //blocking call
}

func (r *runner) newComponentSet(model *reconciler.Reconciliation) *chart.ComponentSet {
	comp := chart.NewComponent(model.Component, model.Namespace, r.configMap(model))
	compSet := chart.NewComponentSet(model.Kubeconfig, model.Version, model.Profile, []*chart.Component{comp})
	return compSet
}

func (r *runner) configMap(model *reconciler.Reconciliation) map[string]interface{} {
	result := make(map[string]interface{}, len(model.Configuration))
	for _, comp := range model.Configuration {
		result[comp.Key] = comp.Value
	}
	return result
}

type LabelInterceptor struct {
}

func (l *LabelInterceptor) Intercept(resource *unstructured.Unstructured) error {
	labels := resource.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[reconciler.ManagedByLabel] = reconciler.LabelReconcilerValue
	resource.SetLabels(labels)
	return nil
}
