package connectivityproxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	v1apps "k8s.io/api/apps/v1"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/connectivityclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	tagHost                        = "global.kubeHost"
	kymaSystem                     = "kyma-system"
	mappingOperatorSecretName      = "connectivity-sm-operator-secrets-tls" // #nosec G101
	mappingsConfigMap              = "connectivity-proxy-service-mappings"
	cpSvcKeySecretName             = "connectivity-proxy-service-key"       // #nosec G101
	smSecretName                   = "connectivity-sm-operator-secrets-tls" // #nosec G101
	versionKey                     = "chart"
	versionToApplyConfigurationFix = "connectivity-proxy-2.9.2"
	configurationConfigMap         = "connectivity-proxy"
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

	certificate, err := a.Commands.CreateSecretMappingOperator(context, kymaSystem)
	if err != nil {
		return fmt.Errorf("unable to create '%s' secret: %w", mappingOperatorSecretName, err)
	}

	err = a.Commands.CreateServiceMappingConfigMap(context, kymaSystem, mappingsConfigMap)
	if err != nil {
		return fmt.Errorf("unable to create '%s' service mapping config map: %w", mappingOperatorSecretName, err)
	}

	secretRootKey, _, err := unstructured.NestedString(binding.Object, "spec", "secretRootKey")
	if err != nil {
		return fmt.Errorf("unable to access binding specification")
	}

	encodedSrk, err := newEncodedSecretSvcKey(secretRootKey, bindingSecret)
	if err != nil {
		return fmt.Errorf("unable to create service_key_secret from %s/%s: %w",
			bindingSecret.Namespace, bindingSecret.Name, err)
	}

	if err := a.Commands.CreateSecretCpSvcKey(context, kymaSystem, cpSvcKeySecretName, encodedSrk); err != nil {
		return fmt.Errorf("unable to create '%s' secret: %w", cpSvcKeySecretName, err)
	}

	if err := prepareOverrides(context, bindingSecret, certificate, secretRootKey); err != nil {
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

	if err := a.Commands.Apply(context, refresh); err != nil {
		return errors.Wrap(err, "Error during reconciliation")
	}

	if err := a.fixConfigurationIfNeeded(context, app); err != nil {
		return errors.Wrap(err, "Error fixing configuration")
	}

	return nil
}

// After the Connectivity Proxy was upgraded to 2.9.2 we must fix the configuration mismatch. After the upgrade the configuration will contain incorrect tunnel's URL (it will start with cc-proxy, not cp as expected)
// As the configuration config map is not applied if it exists (reconciler.kyma-project.io/skip-rendering-on-upgrade annotation), we must update the URL.
// There is no need to perform the fix, if the version installed on the environment is other that 2.9.2
func (a *CustomAction) fixConfigurationIfNeeded(context *service.ActionContext, app *v1apps.StatefulSet) error {
	if app == nil {
		return nil
	}

	labels := app.GetLabels()
	if labels != nil && labels[versionKey] == versionToApplyConfigurationFix {
		context.Logger.Warn("Fixing Connectivity Proxy configuration...")
		return a.Commands.FixConfiguration(context, kymaSystem, configurationConfigMap)
	}

	return nil
}

func newEncodedSecretSvcKey(secretRootKey string, binding *v1.Secret) (string, error) {
	if secretRootKeyProvided := secretRootKey != ""; secretRootKeyProvided {
		data, found := binding.Data[secretRootKey]
		if !found {
			return "", fmt.Errorf("%w: %s", ErrValueNotFound, secretRootKey)
		}
		// workaround for BTP secretRootKey serialization bug
		var s btpSvcKey
		if err := json.Unmarshal(data, &s); err != nil {
			return "", err
		}

		out, err := json.Marshal(&s)
		return string(out), err
	}

	var srk svcKey
	if err := srk.fromSecret(binding); err != nil {
		return "", err
	}

	out, err := json.Marshal(&srk)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type overridePair struct {
	from string
	to   string
}

var (
	ErrValueNotFound          = errors.New("value not found")
	configSubaccountID        = "config.subaccountId"
	configSubaccountSubdomain = "config.subaccountSubdomain"
)

func overrideFromValue(config map[string]interface{}, value []byte) error {
	var data btpSvcKey
	if err := json.Unmarshal(value, &data); err != nil {
		return err
	}

	config[configSubaccountID] = data.SubaccountID
	config[configSubaccountSubdomain] = data.SubaccountSubdomain
	return nil
}

func overrideFromSecret(config map[string]interface{}, secret *v1.Secret) error {
	for _, item := range []overridePair{
		{from: "subaccount_id", to: configSubaccountID},
		{from: "subaccount_subdomain", to: configSubaccountSubdomain},
	} {
		val, found := secret.Data[item.from]
		if !found {
			return fmt.Errorf("%w: %s", ErrValueNotFound, val)
		}
		config[item.to] = string(val)
	}
	return nil
}

func prepareOverrides(actionCtx *service.ActionContext, secret *v1.Secret, caData []byte, secretRootKey string) error {
	overrideSubaccountProperties := func() error {
		return overrideFromSecret(actionCtx.Task.Configuration, secret)
	}

	val, found := secret.Data[secretRootKey]
	if found {
		overrideSubaccountProperties = func() error {
			return overrideFromValue(actionCtx.Task.Configuration, val)
		}
	}

	if err := overrideSubaccountProperties(); err != nil {
		return err
	}

	xtHost := actionCtx.Task.Configuration[tagHost].(string)

	actionCtx.Task.Configuration["config.servers.businessDataTunnel.externalHost"] = fmt.Sprintf("cp.%s", xtHost)
	actionCtx.Task.Configuration["secretConfig.integration.connectivityService.secretName"] = "connectivity-proxy-service-key"
	actionCtx.Task.Configuration["config.servers.businessDataTunnel.externalPort"] = "443"
	actionCtx.Task.Configuration["config.serviceMappings.configMapName"] = mappingsConfigMap
	actionCtx.Task.Configuration["config.serviceMappings.tlsSecret"] = smSecretName

	encoded := base64.StdEncoding.EncodeToString([]byte(caData))
	actionCtx.Task.Configuration["deployment.serviceMapping.caBundle"] = encoded
	return nil
}
