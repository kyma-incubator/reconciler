package connectivityproxy

import (
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/connectivityclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
)

const (
	kymaSystem                = "kyma-system"
	mappingOperatorSecretName = "connectivity-sm-operator-secrets-tls" // #nosec G101
	mappingsConfigMap         = "connectivity-proxy-service-mappings"
)

type CustomAction struct {
	Name     string
	Loader   Loader
	Commands Commands
}

var ErrReconciliationAborted = errors.New("reconciliation aborted")

func (a *CustomAction) Run(context *service.ActionContext) error {
	context.Logger.Debug("Staring invocation of " + context.Task.Component + " reconciliation")

	host := context.KubeClient.GetHost()
	if host == "" {
		return errors.Errorf("Host cannot be empty")
	}
	context.Task.Configuration["global.kubeHost"] = strings.TrimPrefix(host, "https://")

	if context.Task.Type == model.OperationTypeDelete {
		context.Logger.Debug("Requested cluster removal - removing component")
		if err := a.Commands.Remove(context); err != nil {
			context.Logger.Error("Failed to remove Connectivity Proxy: %v", err)
			return err
		}
		return nil
	}

	context.Logger.Debug("Checking StatefulSet")
	app, err := context.KubeClient.GetStatefulSet(context.Context, context.Task.Component, context.Task.Namespace)
	if err != nil {
		return errors.Wrap(err, "Error while retrieving StatefulSet")
	}

	context.Logger.Debug("Checking BTP Operator binding")
	binding, err := a.Loader.FindBindingOperator(context)
	if err != nil {
		return errors.Wrap(err, "Error while retrieving binding from BTP Operator")
	}

	// detect if connectivity-proxy reconciliation should not be skipped

	if binding == nil && app == nil {
		return nil
	}

	// detect if connectivity-proxy is being deleted

	if deleteCP := binding == nil && app != nil; deleteCP {
		context.Logger.Info("Removing component")
		if err := a.Commands.Remove(context); err != nil {
			context.Logger.Error("Failed to remove Connectivity Proxy: %v", err)
			return err
		}
		return nil
	}

	// apply connectivity-proxy

	context.Logger.Debug("Reading ServiceBinding Secret")
	// TODO FindSecret does does not have reference to action and loader
	bindingSecret, err := a.Loader.FindSecret(context, binding)

	context.Logger.Debug("Service Binding Secret check")

	if bindingSecret == nil {
		context.Logger.Warnf("Skipping reconcilion, %s", err)
		return nil
	}

	// build overrides for credential secret by reading them from btp-operator secret
	context.Logger.Debug("Populating configs")
	populateConfigs(context.Task.Configuration, bindingSecret)

	_, err = a.Commands.CreateSecretMappingOperator(context, kymaSystem)
	if err != nil {
		return fmt.Errorf("unable to create '%s' secret: %w", mappingOperatorSecretName, err)
	}

	err = a.Commands.CreateServiceMappingConfigMap(context, kymaSystem, mappingsConfigMap)
	if err != nil {
		return fmt.Errorf("unable to create '%s' service mapping config map: %w", mappingOperatorSecretName, err)
	}

	caClient, err := connectivityclient.NewConnectivityCAClient(context.Task.Configuration)
	if err != nil {
		return errors.Wrap(err, "Error - cannot create Connectivity CA client")
	}

	context.Logger.Debug("Creating Istio CA cacert secret for Connectivity Proxy")
	err = a.Commands.CreateCARootSecret(context, caClient)
	if err != nil {
		return errors.Wrap(err, "error during creatiion of Istio CA cacert secret for Connectivity Proxy")
	}

	refresh := app != nil

	if err := a.Commands.Apply(context, refresh); err != nil {
		return errors.Wrap(err, "Error during reconciliation")
	}

	return nil
}
