package connectivityproxy

import (
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/rendering"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apiCoreV1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	BindingKey             = "global.binding."
	SkipManifestAnnotation = "reconciler.kyma-project.io/skip-rendering-on-upgrade"
)

//go:generate mockery --name=Commands --output=mocks --outpkg=connectivityproxymocks --case=underscore
type Commands interface {
	InstallOrUpgrade(*service.ActionContext, *appsv1.StatefulSet, *apiCoreV1.Secret) error
	CopyResources(*service.ActionContext) error
	Remove(*service.ActionContext) error
	PopulateConfigs(*service.ActionContext, *apiCoreV1.Secret)
}

type NewTargetClientSet func(context *service.ActionContext) (kubernetes.Interface, error)

type CommandActions struct {
	targetClientSetFactory NewTargetClientSet
	install                service.Operation
	copyFactory            []CopyFactory
}

func (a *CommandActions) InstallOrUpgrade(context *service.ActionContext, app *appsv1.StatefulSet, credSecret *apiCoreV1.Secret) error {

	chartProvider, err := a.getChartProvider(context, app, credSecret)

	if err != nil {
		return errors.Wrap(err, "failed to create chart provider")
	}

	err = a.install.Invoke(context.Context, chartProvider, context.Task, context.KubeClient)
	if err != nil {
		return errors.Wrap(err, "failed to invoke installation")
	}

	return nil
}

func (a *CommandActions) getChartProvider(context *service.ActionContext, app *appsv1.StatefulSet, credSecret *apiCoreV1.Secret) (chart.Provider, error) {
	authenticator, err := rendering.NewExternalComponentAuthenticator()
	if err != nil {
		return nil, err
	}
	chartProviderWithAuthentication := rendering.NewProviderWithAuthentication(context.ChartProvider, authenticator)

	filterOutManifests := rendering.NewFilterOutAnnotatedManifests(SkipManifestAnnotation)
	filters := []rendering.FilterFunc{filterOutManifests}
	return rendering.NewProviderWithFilters(chartProviderWithAuthentication, filters...), nil

	return chartProviderWithAuthentication, nil
}

func (a *CommandActions) PopulateConfigs(context *service.ActionContext, bindingSecret *apiCoreV1.Secret) {
	for key, val := range bindingSecret.Data {
		var unmarshalled map[string]interface{}

		if err := json.Unmarshal(val, &unmarshalled); err != nil {
			context.Task.Configuration[BindingKey+key] = string(val)
		} else {
			for uKey, uVal := range unmarshalled {
				context.Task.Configuration[BindingKey+uKey] = uVal
			}
		}
	}
}

func (a *CommandActions) CopyResources(context *service.ActionContext) error {

	clientset, err := a.targetClientSetFactory(context)
	if err != nil {
		return errors.Wrap(err, "Error while getting a client set")
	}

	for _, create := range a.copyFactory {
		operation := create(context.Task, clientset)

		if err := operation.Transfer(); err != nil {
			return err
		}
	}

	return nil
}

func (a *CommandActions) Remove(context *service.ActionContext) error {
	component := chart.NewComponentBuilder(context.Task.Version, context.Task.Component).
		WithNamespace(context.Task.Namespace).
		WithProfile(context.Task.Profile).
		WithConfiguration(context.Task.Configuration).
		WithURL(context.Task.URL).
		Build()

	authenticator, err := rendering.NewExternalComponentAuthenticator()
	if err != nil {
		return errors.Wrap(err, "failed to create chart provider")
	}
	component.SetExternalComponentAuthentication(authenticator)

	manifest, err := context.ChartProvider.RenderManifest(component)
	if err != nil {
		return errors.Wrap(err, "Error during rendering manifest for removal")
	}

	_, err = context.KubeClient.Delete(context.Context, manifest.Manifest, context.Task.Namespace)
	if err != nil {
		return errors.Wrap(err, "Error during removal")
	}

	return a.removeIstioSecrets(context)
}

func (a *CommandActions) removeIstioSecrets(context *service.ActionContext) error {

	_, err := context.KubeClient.DeleteResource(context.Context, "secret", "cc-certs", "istio-system")
	if err != nil {
		context.Logger.Error("Error during removal of cc-certs in istio-system")
		return errors.Wrap(err, "Error during removal of cc-certs in istio-system")
	}

	_, err = context.KubeClient.DeleteResource(context.Context, "secret", "cc-certs-cacert", "istio-system")
	if err != nil {
		context.Logger.Info("Error during removal of cc-certs-cacert in istio-system")
		return errors.Wrap(err, "Error during removal of cc-certs-cacert in istio-system")
	}
	return nil
}
