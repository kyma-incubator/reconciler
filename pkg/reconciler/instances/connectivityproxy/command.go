package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	apiCoreV1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

//go:generate mockery --name=Commands --output=mocks --outpkg=connectivityproxymocks --case=underscore
type Commands interface {
	Install(*service.ActionContext, *apiCoreV1.Secret) error
	CopyResources(context *service.ActionContext) error
	Remove(context *service.ActionContext) error
}

type NewInClusterClientSet func(logger *zap.SugaredLogger) (kubernetes.Interface, error)
type NewTargetClientSet func(context *service.ActionContext) (kubernetes.Interface, error)

type CommandActions struct {
	clientSetFactory       NewInClusterClientSet
	targetClientSetFactory NewTargetClientSet
	install                service.Operation
	copyFactory            []CopyFactory
}

func (a *CommandActions) Install(context *service.ActionContext, bindingSecret *apiCoreV1.Secret) error {
	for key, value := range bindingSecret.Data {
		context.Task.Configuration[key] = value
	}

	err := a.install.Invoke(context.Context, context.ChartProvider, context.Task, context.KubeClient)
	if err != nil {
		return errors.Wrap(err, "Error during installation")
	}

	return nil
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
		operation := create(context.Task.Configuration, inCluster, clientset)

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
