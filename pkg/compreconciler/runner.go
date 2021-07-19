package compreconciler

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/chart"
)

type runner struct {
	*ComponentReconciler
}

func (r *runner) Run(ctx context.Context, model *Reconciliation, callback CallbackHandler) error {
	statusUpdater := newStatusUpdater(ctx, r.updateInterval, callback, uint(r.maxRetries), r.debug)

	retryable := func(statusUpdater *StatusUpdater) func() error {
		return func() error {
			if err := statusUpdater.Running(); err != nil {
				return err
			}
			err := r.reconcile(model)
			if err != nil {
				if err := statusUpdater.Failed(); err != nil {
					return err
				}
			}
			return err
		}
	}(statusUpdater)

	//retry the reconciliation in case of an error
	err := retry.Do(retryable,
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

func (r *runner) reconcile(model *Reconciliation) error {
	kubeClient, err := NewClient(model.Kubeconfig)
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
		if err := r.install(model, kubeClient); err != nil {
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

func (r *runner) install(model *Reconciliation, kubeClient Client) error {
	manifest, err := r.renderManifest(model)
	if err != nil {
		return err
	}

	if err := kubeClient.Deploy(manifest); err != nil {
		return err
	}

	return r.trackProgress(manifest, kubeClient) //blocking call
}

func (r *runner) renderManifest(model *Reconciliation) (string, error) {
	manifests, err := r.chartProvider.Manifests(r.newComponentSet(model), &chart.Options{})
	if err != nil {
		return "", err
	}

	if len(manifests) != 1 { //just an assertion - can in current implementation not occur
		return "", fmt.Errorf("reconciliation can only process 1 manifest but got %d", len(manifests))
	}
	return manifests[0].Manifest, nil
}

func (r *runner) trackProgress(manifest string, kubeClient Client) error {
	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}
	//get resources defined in manifest
	pt, err := NewProgressTracker(clientSet, r.debug, ProgressTrackerConfig{})
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
			return err
		}
		pt.AddResource(
			watchable,
			resource.namespace,
			resource.name,
		)
	}
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
