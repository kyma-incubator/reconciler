package connectivityproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/connectivityclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/secrets"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	apiCoreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

type CustomAction struct {
	Name     string
	Loader   Loader
	Commands Commands
}

const (
	tagHost            = "global.kubeHost"
	smSecretName       = "connectivity-sm-operator-secrets-tls"
	cpSvcKeySecretName = "connectivity-proxy-service-key"
	kymaSystem         = "kyma-system"
	mappingsConfigMap  = "connectivity-proxy-service-mappings"
)

func (a *CustomAction) Run(actionCtx *service.ActionContext) error {
	actionCtx.Logger.Debug("Staring invocation of " + actionCtx.Task.Component + " reconciliation")

	host := actionCtx.KubeClient.GetHost()
	if host == "" {
		return errors.Errorf("Host cannot be empty")
	}
	actionCtx.Task.Configuration[tagHost] = strings.TrimPrefix(host, "https://")

	if actionCtx.Task.Type == model.OperationTypeDelete {
		actionCtx.Logger.Debug("Requested cluster removal - removing component")
		if err := a.Commands.Remove(actionCtx); err != nil {
			actionCtx.Logger.Error("Failed to remove Connectivity Proxy: %v", err)
			return err
		}
		return nil
	}

	actionCtx.Logger.Debug("Checking StatefulSet")
	app, err := actionCtx.KubeClient.GetStatefulSet(actionCtx.Context, actionCtx.Task.Component, actionCtx.Task.Namespace)
	if err != nil {
		return errors.Wrap(err, "Error while retrieving StatefulSet")
	}

	actionCtx.Logger.Debug("Checking BTP Operator binding")
	binding, err := a.Loader.FindBindingOperator(actionCtx)
	if err != nil {
		return errors.Wrap(err, "Error while retrieving binding from BTP Operator")
	}

	if binding != nil {

		actionCtx.Logger.Debug("Reading ServiceBinding Secret")
		bindingSecret, err := a.Loader.FindSecret(actionCtx, binding)

		actionCtx.Logger.Debug("Service Binding Secret check")
		if err != nil {
			return errors.Wrap(err, "Error while retrieving service binding secret")
		}

		// TODO rethink binding secret retrieval
		if bindingSecret == nil {
			return errors.New("Missing binding secret")
		}

		// build overrides for credential secret by reading them from btp-operator secret
		actionCtx.Logger.Debug("Populating configs")

		// TODO this is a workaround for 2.4.4, clean it up after upgrade to 2.8.0
		a.Commands.PopulateConfigs(actionCtx, bindingSecret)

		data, err := a.Commands.CreateSecretTLS(actionCtx, kymaSystem, smSecretName)
		if err != nil {
			return fmt.Errorf("unable to create '%s' secret: %w", smSecretName, err)
		}

		err = a.Commands.CreateServiceMappingConfigMap(actionCtx, kymaSystem, mappingsConfigMap)
		if err != nil {
			return fmt.Errorf("unable to create '%s' service mapping config map: %w", smSecretName, err)
		}

		encodedSrk, err := newEncodedSecretSvcKey(bindingSecret)
		if err != nil {
			return fmt.Errorf("unable to create service_key_secret from %s/%s: %w",
				bindingSecret.Namespace, bindingSecret.Name, err)
		}

		if err := a.Commands.CreateSecretCpSvcKey(actionCtx, kymaSystem, cpSvcKeySecretName, encodedSrk); err != nil {
			return fmt.Errorf("unable to create '%s' secret: %w", cpSvcKeySecretName, err)
		}

		caData, found := data[secrets.TagTlsCa]
		if !found {
			return fmt.Errorf("not found: %s in %s/%s", secrets.TagTlsCa, kymaSystem, smSecretName)
		}

		if err := prepareOverridesFor280(actionCtx, bindingSecret, caData); err != nil {
			return errors.Wrap(err, "Error - cannot prepare overrides")
		}

		caClient, err := connectivityclient.NewConnectivityCAClient(actionCtx.Task.Configuration)

		if err != nil {
			return errors.Wrap(err, "Error - cannot create Connectivity CA client")
		}
		actionCtx.Logger.Debug("Creating Istio CA cacert secret for Connectivity Proxy")
		err = a.Commands.CreateCARootSecret(actionCtx, caClient)
		if err != nil {
			return errors.Wrap(err, "error during creatiion of Istio CA cacert secret for Connectivity Proxy")
		}

		refresh := app != nil

		actionCtx.Logger.Info("Installing component")

		if err := a.Commands.Apply(actionCtx, refresh); err != nil {
			return errors.Wrap(err, "Error during reconcilation")
		}
	} else if binding == nil && app != nil {
		actionCtx.Logger.Info("Removing component")
		if err := a.Commands.Remove(actionCtx); err != nil {
			actionCtx.Logger.Error("Failed to remove Connectivity Proxy: %v", err)
			return err
		}
	}

	return nil
}

var (
	ErrValueNotFound = errors.New("value not found")
)

func prepareOverridesFor280(actionCtx *service.ActionContext, secret *apiCoreV1.Secret, caData []byte) error {
	for _, item := range [][2]string{
		{"subaccount_id", "config.subaccountId"},
		{"subaccount_subdomain", "config.subaccountSubdomain"},
	} {
		val, found := secret.Data[item[0]]
		if !found {
			return fmt.Errorf("%w: %s", ErrValueNotFound, val)
		}
		actionCtx.Task.Configuration[item[1]] = string(val)
	}

	xtHost := actionCtx.Task.Configuration[tagHost].(string)

	actionCtx.Task.Configuration["config.servers.businessDataTunnel.externalHost"] = fmt.Sprintf("cc-proxy.%s", xtHost)
	actionCtx.Task.Configuration["secretConfig.integration.connectivityService.secretName"] = "connectivity-proxy-service-key"
	actionCtx.Task.Configuration["config.servers.businessDataTunnel.externalPort"] = "443"
	actionCtx.Task.Configuration["config.serviceMappings.configMapName"] = mappingsConfigMap
	actionCtx.Task.Configuration["config.serviceMappings.tlsSecret"] = smSecretName

	encoded := base64.StdEncoding.EncodeToString([]byte(caData))
	actionCtx.Task.Configuration["deployment.serviceMapping.caBundle"] = encoded
	return nil
}

type encodedString string

func (s *encodedString) UnmarshalJSON(src []byte) error {
	sanitized := bytes.Trim(src, "\"")
	raw, err := base64.StdEncoding.DecodeString(string(sanitized))
	if err != nil {
		return err
	}

	*s = encodedString(raw)
	return nil
}

type encodedSlice []encodedString

func (s *encodedSlice) UnmarshalJSON(src []byte) error {
	sanitized := bytes.Trim(src, "\"")
	decodedSrc, err := base64.StdEncoding.DecodeString(string(sanitized))
	if err != nil {
		return err
	}

	var data []string
	if err := json.Unmarshal(decodedSrc, &data); err != nil {
		return err
	}

	out := []encodedString{}
	for _, item := range data {
		out = append(out, encodedString(item))
	}
	*s = out
	return nil
}

type connectivitySvc struct {
	CasPath       string `json:"CAs_path"`
	CasStringPath string `json:"CAs_signing_path"`
	ApiPath       string `json:"api_path"`
	TunnelPath    string `json:"tunnel_path"`
	URL           string `json:"url"`
}

func (s *connectivitySvc) UnmarshalJSON(src []byte) error {
	sanitized := bytes.Trim(src, "\"")
	raw, err := base64.StdEncoding.DecodeString(string(sanitized))
	if err != nil {
		return err
	}
	// create alias not to fall into circular unmarshal
	type connectivitySvcAlias connectivitySvc
	var cpAlias connectivitySvcAlias

	if err := json.Unmarshal(raw, &cpAlias); err != nil {
		return err
	}

	*s = connectivitySvc(cpAlias)
	return nil
}

type svcKey struct {
	ClientID                    encodedString   `json:"clientid"`
	ClientSecret                encodedString   `json:"clientsecret"`
	ConnectovitySvc             connectivitySvc `json:"connectivity_service"`
	CredentialsType             encodedString   `json:"credential-type"`
	InstanceGUID                encodedString   `json:"instance_guid"`
	InstanceName                encodedString   `json:"instance_name"`
	Label                       encodedString   `json:"label"`
	Plan                        encodedString   `json:"plan"`
	SubaccountID                encodedString   `json:"subaccount_id"`
	SubaccountSubdomain         encodedString   `json:"subaccount_subdomain"`
	Tags                        encodedSlice    `json:"tags"`
	TokenSvcDomain              encodedString   `json:"token_service_domain"`
	TokenSvcURL                 encodedString   `json:"token_service_url"`
	TokenSvcURLPattern          encodedString   `json:"token_service_url_pattern"`
	TokenSVCURLPatternTenantKey encodedString   `json:"token_service_url_pattern_tenant_key"`
	Type                        encodedString   `json:"type"`
	XsAppName                   encodedString   `json:"xsappname"`
}

func (k *svcKey) fromSecret(s *v1.Secret) error {
	data, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, k)
}

func newEncodedSecretSvcKey(binding *v1.Secret) (string, error) {
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
