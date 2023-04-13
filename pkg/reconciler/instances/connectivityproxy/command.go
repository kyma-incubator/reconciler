package connectivityproxy

import (
	"encoding/json"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/connectivityclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/rendering"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/secrets"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	apiCoreV1 "k8s.io/api/core/v1"
	k8s "k8s.io/client-go/kubernetes"
)

const (
	BindingKey             = "global.binding."
	SkipManifestAnnotation = "reconciler.kyma-project.io/skip-rendering-on-upgrade"
)

//go:generate mockery --name=Commands --output=mocks --outpkg=connectivityproxymocks --case=underscore
type Commands interface {
	Apply(*service.ActionContext, bool) error
	CreateCARootSecret(*service.ActionContext, connectivityclient.ConnectivityClient) error
	Remove(*service.ActionContext) error
	PopulateConfigs(*service.ActionContext, *apiCoreV1.Secret)
}

type CommandActions struct {
	install service.Operation
}

func (a *CommandActions) Apply(context *service.ActionContext, refresh bool) error {

	chartProvider, err := a.getChartProvider(context, refresh)

	if err != nil {
		return errors.Wrap(err, "failed to create chart provider")
	}

	err = a.install.Invoke(context.Context, chartProvider, context.Task, context.KubeClient)
	if err != nil {
		return errors.Wrap(err, "failed to invoke installation")
	}

	return nil
}

func (a *CommandActions) getChartProvider(context *service.ActionContext, withFilter bool) (chart.Provider, error) {
	authenticator, err := rendering.NewExternalComponentAuthenticator()
	if err != nil {
		return nil, err
	}
	chartProviderWithAuthentication := rendering.NewProviderWithAuthentication(context.ChartProvider, authenticator)

	var filters []rendering.FilterFunc

	if withFilter {
		filterOutManifests := rendering.NewFilterOutAnnotatedManifests(SkipManifestAnnotation)
		filters = append(filters, filterOutManifests)
	}

	return rendering.NewProviderWithFilters(chartProviderWithAuthentication, filters...), nil
}

func (a *CommandActions) PopulateConfigs(context *service.ActionContext, bindingSecret *apiCoreV1.Secret) {
	for key, val := range bindingSecret.Data {
		a.flatValues(context, key, val)
	}
}

func (a *CommandActions) flatValues(context *service.ActionContext, key string, value []byte) {

	var unmarshalled map[string]interface{}

	if err := json.Unmarshal(value, &unmarshalled); err != nil {
		context.Task.Configuration[BindingKey+key] = string(value)
	} else {
		for uKey, uVal := range unmarshalled {

			strVal, ok := uVal.(string)
			if ok {
				a.flatValues(context, uKey, []byte(strVal))
			} else {
				context.Task.Configuration[BindingKey+uKey] = uVal
			}
		}
	}
}

func (a *CommandActions) CreateCARootSecret(context *service.ActionContext, caClient connectivityclient.ConnectivityClient) error {

	ca, err := caClient.GetCA()

	if err != nil {
		return err
	}

	clientset, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "cannot get a target cluster client set")
	}

	if err := makeIstioCASecret(context.Task, clientset, ca); err != nil {
		return errors.Wrap(err, "cannot create Istio CA secret")
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

func makeIstioCASecret(task *reconciler.Task, targetClientSet k8s.Interface, ca []byte) error {

	namespace, secretKey, secretName, err := getIstioSecretCfg(task.Configuration)

	if err != nil {
		return err
	}

	repo := secrets.NewSecretRepo(namespace, targetClientSet)
	return repo.SaveIstioCASecret(secretName, secretKey, ca)
}

func getIstioSecretCfg(config map[string]interface{}) (string, string, string, error) {
	istioSecretName, ok := config["istio.secret.name"]

	if !ok {
		return "", "", "", errors.New("missing configuration value istio.secret.name")
	}

	istioNamespace := config["istio.secret.namespace"]
	if istioNamespace == nil || istioNamespace == "" {
		istioNamespace = "istio-system"
	}
	istioSecretKey := config["istio.secret.key"]
	if istioSecretKey == nil || istioSecretKey == "" {
		istioSecretKey = "cacert"
	}

	strNamespace := fmt.Sprintf("%v", istioNamespace)
	strSecretKey := fmt.Sprintf("%v", istioSecretKey)
	strSecretName := fmt.Sprintf("%v", istioSecretName)

	return strNamespace, strSecretKey, strSecretName, nil
}
