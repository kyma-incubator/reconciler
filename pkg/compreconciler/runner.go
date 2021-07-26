package compreconciler

import (
	"bytes"
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/reconciler/pkg/chart"
	"github.com/pkg/errors"
)

type runner struct {
	*ComponentReconciler
}

func (r *runner) Run(ctx context.Context, model *Reconciliation, callback CallbackHandler) error {
	statusUpdater, err := newStatusUpdater(ctx, callback, r.debug, StatusUpdaterConfig{
		Interval:   r.statusUpdaterConfig.interval,
		MaxRetries: r.statusUpdaterConfig.maxRetries,
		RetryDelay: r.statusUpdaterConfig.retryDelay,
	})
	if err != nil {
		return err
	}

	retryable := func(statusUpdater *StatusUpdater) func() error {
		return func() error {
			if err := statusUpdater.Running(); err != nil {
				return err
			}
			err := r.reconcile(ctx, model)
			if err != nil {
				if err := statusUpdater.Failed(); err != nil {
					return err
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

	logger := r.logger()
	if err == nil {
		logger.Info(
			fmt.Sprintf("Reconciliation of component '%s' for version '%s' finished successfully",
				model.Component, model.Version))
		if err := statusUpdater.Success(); err != nil {
			return err
		}
	} else {
		logger.Warn(
			fmt.Sprintf("Retryable reconciliation of component '%s' for version '%s' failed consistently: giving up",
				model.Component, model.Version))
		if err := statusUpdater.Error(); err != nil {
			return err
		}
	}

	return err
}

func (r *runner) reconcile(ctx context.Context, model *Reconciliation) error {
	kubeClient, err := newKubernetesClient(model.Kubeconfig)
	if err != nil {
		return err
	}

	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	logger := r.logger()
	if r.preInstallAction != nil {
		if err := r.preInstallAction.Run(model.Version, clientSet); err != nil {
			logger.Warn(
				fmt.Sprintf("Pre-installation action of version '%s' failed: %s", model.Version, err))
			return err
		}
	}

	if r.installAction == nil {
		if err := r.install(ctx, model, kubeClient); err != nil {
			logger.Warn(
				fmt.Sprintf("Default-installation of version '%s' failed: %s", model.Version, err))
			return err
		}
	} else {
		if err := r.installAction.Run(model.Version, clientSet); err != nil {
			logger.Warn(
				fmt.Sprintf("Installation action of version '%s' failed: %s", model.Version, err))
			return err
		}
	}

	if r.postInstallAction != nil {
		if err := r.postInstallAction.Run(model.Version, clientSet); err != nil {
			logger.Warn(
				fmt.Sprintf("Post-installation action of version '%s' failed: %s", model.Version, err))
			return err
		}
	}

	return nil
}

func (r *runner) install(ctx context.Context, model *Reconciliation, kubeClient kubernetesClient) error {
	manifest, err := r.renderManifest(model)
	if err != nil {
		return err
	}

	if err := kubeClient.Deploy(manifest); err != nil {
		r.logger().Warn(fmt.Sprintf("Failed to deploy manifests on target cluster: %s", err))
		return err
	}

	return r.trackProgress(ctx, manifest, kubeClient) //blocking call
}

func (r *runner) renderManifest(model *Reconciliation) (string, error) {
	manifests, err := r.chartProvider.Manifests(r.newComponentSet(model), model.InstallCRD, &chart.Options{})
	if err != nil {
		msg := fmt.Sprintf("Failed to render manifest for component '%s'", model.Component)
		r.logger().Warn(msg)
		return "", errors.Wrap(err, msg)
	}

	var buffer bytes.Buffer
	r.logger().Debug(fmt.Sprintf("Rendering of component '%s' returned %d manifests", model.Component, len(manifests)))
	for _, manifest := range manifests {
		if !model.InstallCRD && manifest.Type == components.CRD {
			r.logger().Error(fmt.Sprintf("Illegal state detected! "+
				"No CRDs were requested but chartProvider returned CRD manifest: '%s'", manifest.Name))
		}
		buffer.WriteString("---\n")
		buffer.WriteString(fmt.Sprintf("# Manifest of %s '%s'\n", manifest.Type, model.Component))
		buffer.WriteString(manifest.Manifest)
		buffer.WriteString("\n")
	}
	return buffer.String(), nil
}

func (r *runner) trackProgress(ctx context.Context, manifest string, kubeClient kubernetesClient) error {
	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}
	//get resources defined in manifest
	pt, err := NewProgressTracker(ctx, clientSet, r.debug, ProgressTrackerConfig{
		timeout:  r.progressTrackerConfig.timeout,
		interval: r.progressTrackerConfig.interval,
	})
	if err != nil {
		return err
	}
	//watch progress of installed resources
	resources, err := kubeClient.DeployedResources(manifest)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		watchable, err := NewWatchableResource(resource.kind) //convert "kind" to watchable
		if err != nil {
			pt.logger.Debug(fmt.Sprintf("Ignoring non-watchable resource '%s' (%s:%s)",
				resource.kind, resource.name, resource.namespace))
			continue //not watchable resource: ignore it
		}
		pt.AddResource(
			watchable,
			resource.namespace,
			resource.name,
		)
	}
	r.logger().Debug("Start watching installation progress")
	return pt.Watch() //blocking call
}

func (r *runner) newComponentSet(model *Reconciliation) *chart.ComponentSet {
	comp := chart.NewComponent(model.Component, model.Namespace, r.configMap(model))
	compSet := chart.NewComponentSet(model.Kubeconfig, model.Version, model.Profile, []*chart.Component{comp})
	return compSet
}

func (r *runner) configMap(model *Reconciliation) map[string]interface{} {
	result := make(map[string]interface{}, len(model.Configuration))
	for _, comp := range model.Configuration {
		result[comp.Key] = comp.Value
	}
	return result
}
