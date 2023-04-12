package connectivityproxy

import (
	"fmt"
	"strings"

	"encoding/base64"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/connectivityclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/secrets"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	apiCoreV1 "k8s.io/api/core/v1"
)

type CustomAction struct {
	Name     string
	Loader   Loader
	Commands Commands
}

const (
	tagHost      = "global.kubeHost"
	smSecretName = "connectivity-sm-operator-secrets-tls"
	kymaSystem   = "kyma-system"
)

func (a *CustomAction) Run(context *service.ActionContext) error {
	context.Logger.Debug("Staring invocation of " + context.Task.Component + " reconciliation")

	host := context.KubeClient.GetHost()
	if host == "" {
		return errors.Errorf("Host cannot be empty")
	}
	context.Task.Configuration[tagHost] = strings.TrimPrefix(host, "https://")

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

	if binding != nil {
		context.Logger.Debug("Reading ServiceBinding Secret")
		bindingSecret, err := a.Loader.FindSecret(context, binding)

		context.Logger.Debug("Service Binding Secret check")
		if err != nil {
			return errors.Wrap(err, "Error while retrieving service binding secret")
		}

		// TODO rethink binding secret retrieval
		if bindingSecret == nil {
			return errors.New("Missing binding secret")
		}

		// build overrides for credential secret by reading them from btp-operator secret
		context.Logger.Debug("Populating configs")

		// TODO this is a workaround for 2.4.4, clean it up after upgrade to 2.8.0
		a.Commands.PopulateConfigs(context, bindingSecret)

		data, err := a.Commands.CreateSecretTLS(context, kymaSystem, smSecretName)
		if err != nil {
			return fmt.Errorf("unable to create '%s' secret: %w", smSecretName, err)
		}

		caData, found := data[secrets.TagTlsCa]
		if !found {
			return fmt.Errorf("not found: %s in %s/%s", secrets.TagTlsCa, kymaSystem, smSecretName)
		}

		if err := prepareOverridesFor280(context, bindingSecret, caData); err != nil {
			return errors.Wrap(err, "Error - cannot prepare overrides")
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

		if refresh {
			context.Logger.Info("Reconciling component")
		} else {
			context.Logger.Info("Installing component")
		}

		if err := a.Commands.Apply(context, refresh); err != nil {
			return errors.Wrap(err, "Error during reconcilation")
		}
	} else if binding == nil && app != nil {
		context.Logger.Info("Removing component")
		if err := a.Commands.Remove(context); err != nil {
			context.Logger.Error("Failed to remove Connectivity Proxy: %v", err)
			return err
		}
	}

	return nil
}

var (
	ErrValueNotFound = errors.New("value not found")
)

func prepareOverridesFor280(context *service.ActionContext, secret *apiCoreV1.Secret, caData []byte) error {
	for _, item := range [][2]string{
		{"subaccount_id", "config.subaccountId"},
		{"subaccount_subdomain", "config.subaccountSubdomain"},
	} {
		val, found := secret.Data[item[0]]
		if !found {
			return fmt.Errorf("%w: %s", ErrValueNotFound, val)
		}
		context.Task.Configuration[item[1]] = string(val)
	}

	context.Task.Configuration["config.servers.businessDataTunnel.externalHost"] = fmt.Sprintf("conn.%s", context.Task.Configuration[tagHost])
	context.Task.Configuration["secretConfig.integration.connectivityService.secretName"] = "connectivity-proxy-service-key"

	encoded := base64.StdEncoding.EncodeToString([]byte(caData))
	context.Task.Configuration["deployment.serviceMapping.caBundle"] = encoded
	return nil
}
