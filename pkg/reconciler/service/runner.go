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
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/adapter"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
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
			if err != nil {
				r.logger.Warnf("Failing reconciliation of '%s' in version '%s' with profile '%s': %s",
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
	kubeClient, err := adapter.NewKubernetesClient(model.Kubeconfig, r.logger, &adapter.Config{
		ProgressInterval: r.progressTrackerConfig.interval,
		ProgressTimeout:  r.progressTrackerConfig.timeout,
	})
	if err != nil {
		return err
	}

	inClusterClient, err := kubeclient.NewInCluster()
	if err != nil {
		return err
	}

	inClusterClientSet, err := inClusterClient.GetClientSet()
	if err != nil {
		return err
	}

	clusterClientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	configs := model.ConfigsToMap()
	err = model.Repo.ReadToken(inClusterClientSet.CoreV1(), configs["url.namespace"])
	if err != nil {
		return err
	}

	chartProvider, err := r.newChartProvider(&model.Repo)
	if err != nil {
		return errors.Wrap(err, "Failed to create chart provider instance")
	}

	factory := r.workspaceFactory(&model.Repo)
	if err != nil {
		return err
	}
	actionHelper := &ActionContext{
		InClusterClientSet: inClusterClientSet,
		KubeClient:         kubeClient,
		ClientSet:          clusterClientSet,
		WorkspaceFactory:   factory,
		Context:            ctx,
		Logger:             r.logger,
		ChartProvider:      chartProvider,
		ConfigsMap:         configs,
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
