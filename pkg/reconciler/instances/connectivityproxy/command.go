package connectivityproxy

import (
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/rendering"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	apiCoreV1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"os"
)

const (
	BindingKey             = "global.binding."
	SkipManifestAnnotation = "reconciler.kyma-project.io/skip-rendering-on-upgrade"
)

//go:generate mockery --name=Commands --output=mocks --outpkg=connectivityproxymocks --case=underscore
type Commands interface {
	InstallOrUpgrade(*service.ActionContext, *appsv1.StatefulSet) error
	CopyResources(*service.ActionContext) error
	Remove(*service.ActionContext) error
	PopulateConfigs(*service.ActionContext, *apiCoreV1.Secret)
}

type NewInClusterClientSet func(logger *zap.SugaredLogger) (kubernetes.Interface, error)
type NewTargetClientSet func(context *service.ActionContext) (kubernetes.Interface, error)

type CommandActions struct {
	clientSetFactory       NewInClusterClientSet
	targetClientSetFactory NewTargetClientSet
	install                service.Operation
	copyFactory            []CopyFactory
}

func (a *CommandActions) InstallOrUpgrade(context *service.ActionContext, app *appsv1.StatefulSet) error {

	chartDownloadToken := os.Getenv("GIT_CLONE_TOKEN") //#nosec [-- Ignore nosec false positive. It's not a credential, just an environment variable name]
	if chartDownloadToken == "" {
		return errors.New("failed to get chart download access token")
	}
	chartProvider := a.getChartProvider(context, app, chartDownloadToken)

	err := a.install.Invoke(context.Context, chartProvider, context.Task, context.KubeClient)
	if err != nil {
		return errors.Wrap(err, "failed to invoke installation")
	}

	return nil
}

func (a *CommandActions) getChartProvider(context *service.ActionContext, app *appsv1.StatefulSet, chartDownloadToken string) chart.Provider {
	authenticator := rendering.NewExternalComponentAuthenticator(chartDownloadToken)
	chartProviderWithAuthentication := rendering.NewProviderWithHttpAuthentication(context.ChartProvider, authenticator)

	upgrade := app != nil && app.GetLabels() != nil

	if upgrade {
		filterOutManifests := rendering.NewFilterOutAnnotatedManifests(SkipManifestAnnotation)
		skipInstallationIfReleaseNotChanged := rendering.NewSkipReinstallingCurrentRelease(context.Logger, app.Name, app.GetLabels()[rendering.ReleaseLabelKey])
		filters := []rendering.FilterFunc{skipInstallationIfReleaseNotChanged, filterOutManifests}

		return rendering.NewProviderWithFilters(chartProviderWithAuthentication, filters...)
	}

	return chartProviderWithAuthentication
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
	inCluster, err := a.clientSetFactory(context.Logger)
	if err != nil {
		return err
	}

	clientset, err := a.targetClientSetFactory(context)
	if err != nil {
		return errors.Wrap(err, "Error while getting a client set")
	}

	for _, create := range a.copyFactory {
		operation := create(context.Task, inCluster, clientset)

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
