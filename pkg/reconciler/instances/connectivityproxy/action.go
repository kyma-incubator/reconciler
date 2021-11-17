package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
)

type CustomAction struct {
	Name     string
	Loader   Loader
	Commands Commands
}

func (a *CustomAction) Run(context *service.ActionContext) error {
	context.Logger.Info("Staring invocation of " + context.Task.Component + " reconciliation")

	context.Logger.Info("Checking statefulset")
	app, err := context.KubeClient.
		GetStatefulSet(context.Context, context.Task.Component, context.Task.Namespace)
	if err != nil {
		return errors.Wrap(err, "Error while retrieving app")
	}

	context.Logger.Info("Checking operator binding")
	binding, err := a.Loader.FindBindingOperator(context)
	if err != nil {
		return errors.Wrap(err, "Error while retrieving binding from BTP")
	}

	if binding == nil {
		context.Logger.Info("Checking catalog binding")
		binding, err = a.Loader.FindBindingCatalog(context)
		if err != nil {
			return errors.Wrap(err, "Error while retrieving binding from Service Catalog")
		}
	}

	if binding != nil && app == nil {
		context.Logger.Info("Reading secret")
		bindingSecret, err := a.Loader.FindSecret(context, binding)
		if err != nil {
			return errors.Wrap(err, "Error while retrieving secret")
		}

		context.Logger.Info("Copying resources to target cluster")
		err = a.Commands.CopyResources(context)
		if err != nil {
			return errors.Wrap(err, "Error during copying resources")
		}

		context.Logger.Info("Installing component")
		if err := a.Commands.Install(context, bindingSecret); err != nil {
			return errors.Wrap(err, "Error during installation")
		}
	} else if binding == nil && app != nil {
		context.Logger.Info("Removing component")
		if err := a.Commands.Remove(context); err != nil {
			return err
		}
	}

	return nil
}
