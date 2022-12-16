package connectivityproxy

import (
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	apiCoreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

const (
	BindingKey            = "global.binding."
	ReleaseLabelKey       = "release"
	ConnectivityProxyKind = "StatefulSet"
)

//go:generate mockery --name=Commands --output=mocks --outpkg=connectivityproxymocks --case=underscore
type Commands interface {
	InstallOnReleaseChange(*service.ActionContext, *appsv1.StatefulSet) error
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

func (a *CommandActions) InstallOnReleaseChange(context *service.ActionContext, app *appsv1.StatefulSet) error {
	filterOutConfigMap := func(unstructs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		newUnstructs := make([]*unstructured.Unstructured, 0, 0)

		for _, unstruct := range unstructs {
			annotations := unstruct.GetAnnotations()
			_, ok := annotations["skip"]
			if !ok {
				newUnstructs = append(newUnstructs, unstruct)
			}
		}

		return newUnstructs, nil
	}

	if app == nil || (app != nil && app.GetLabels() == nil) {
		context.Logger.Debug("There is no valid Connectivity Proxy installed, invoking the installation")
		newChartProvider := NewProviderWithFilters(context.ChartProvider, filterOutConfigMap)

		return a.installOnCondition(context, newChartProvider)
	}

	appName := app.Name
	appRelease := app.GetLabels()[ReleaseLabelKey]

	filterOutIfReleaseDiffers := func(unstructs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		var statefulSetManifest *unstructured.Unstructured
		for _, unstruct := range unstructs {
			if unstruct != nil && unstruct.GetName() == appName && unstruct.GetKind() == ConnectivityProxyKind {
				statefulSetManifest = unstruct
				break
			}
		}

		if statefulSetManifest == nil {
			context.Logger.Warn("Did not find the Connectivity Proxy stateful set, skipping")
			return nil, errors.Errorf("Connectivity Proxy stateful set does not have any release labels")
		}

		if statefulSetManifest.GetLabels() == nil || statefulSetManifest.GetLabels()[ReleaseLabelKey] == "" {
			return nil, errors.Errorf("Connectivity Proxy StatefulSet does not have any release labels")
		}

		if statefulSetManifest.GetLabels()[ReleaseLabelKey] != appRelease {
			context.Logger.Debug("Connectivity Proxy release has changed, the component will be upgraded")
			return unstructs, nil
		}

		context.Logger.Debug("Connectivity Proxy release did not change, skipping")
		return nil, nil
	}

	newChartProvider := NewProviderWithFilters(context.ChartProvider, filterOutIfReleaseDiffers, filterOutConfigMap)

	return a.installOnCondition(context, newChartProvider)
}

func (a *CommandActions) installOnCondition(context *service.ActionContext, chartProvider chart.Provider) error {
	err := a.install.Invoke(context.Context, chartProvider, context.Task, context.KubeClient)
	if err != nil {
		return errors.Wrap(err, "failed to invoke conditional installation")
	}

	return nil
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
