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

const BindingKey = "global.binding."

//go:generate mockery --name=Commands --output=mocks --outpkg=connectivityproxymocks --case=underscore
type Commands interface {
	InstallIfOther(*service.ActionContext, *appsv1.StatefulSet) error
	Install(*service.ActionContext) error
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
	iterate                service.ManifestLookup
	copyFactory            []CopyFactory
}

func (a *CommandActions) InstallIfOther(context *service.ActionContext, app *appsv1.StatefulSet) error {
	if app == nil {
		return a.Install(context)
	}

	found, err := a.iterate.Lookup(func(unstructured *unstructured.Unstructured) bool {
		return unstructured != nil &&
			unstructured.GetName() != "" && unstructured.GetName() == app.Name &&
			unstructured.GetNamespace() != "" && unstructured.GetNamespace() == app.Namespace
	}, context.ChartProvider, context.Task)

	if err != nil {
		return err
	}

	if found != nil && found.GetLabels() == nil ||
		found.GetLabels()["release"] == "" ||
		app.GetLabels() == nil ||
		app.GetLabels()["release"] == "" {
		return errors.New("Invalid state, missing release label")
	}

	if found != nil && found.GetLabels()["release"] == app.GetLabels()["release"] {
		context.Logger.Infof("New version, update skipped")
		return nil
	}

	return a.Install(context)
}

func (a *CommandActions) Install(context *service.ActionContext) error {
	err := a.install.Invoke(context.Context, context.ChartProvider, context.Task, context.KubeClient)
	if err != nil {
		return errors.Wrap(err, "Error during installation")
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
	return nil
}
