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
	"github.com/kyma-incubator/reconciler/pkg/ssl"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
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
	CreateSecretCpSvcKey(ctx *service.ActionContext, ns, secretName, cpSvcKey string) error
	CreateSecretTLS(ctx *service.ActionContext, ns, secretName string) (map[string][]byte, error)
	Remove(*service.ActionContext) error
	PopulateConfigs(*service.ActionContext, *apiCoreV1.Secret)
}

type CommandActions struct {
	install service.Operation
}

func (a *CommandActions) Apply(ctx *service.ActionContext, refresh bool) error {

	chartProvider, err := a.getChartProvider(ctx, refresh)

	if err != nil {
		return errors.Wrap(err, "failed to create chart provider")
	}

	err = a.install.Invoke(ctx.Context, chartProvider, ctx.Task, ctx.KubeClient)
	if err != nil {
		return errors.Wrap(err, "failed to invoke installation")
	}

	return nil
}

func (a *CommandActions) getChartProvider(ctx *service.ActionContext, withFilter bool) (chart.Provider, error) {
	authenticator, err := rendering.NewExternalComponentAuthenticator()
	if err != nil {
		return nil, err
	}
	chartProviderWithAuthentication := rendering.NewProviderWithAuthentication(ctx.ChartProvider, authenticator)

	var filters []rendering.FilterFunc

	if withFilter {
		filterOutManifests := rendering.NewFilterOutAnnotatedManifests(SkipManifestAnnotation)
		filters = append(filters, filterOutManifests)
	}

	return rendering.NewProviderWithFilters(chartProviderWithAuthentication, filters...), nil
}

func (a *CommandActions) PopulateConfigs(ctx *service.ActionContext, bindingSecret *apiCoreV1.Secret) {
	for key, val := range bindingSecret.Data {
		var unmarshalled map[string]interface{}

		if err := json.Unmarshal(val, &unmarshalled); err != nil {
			ctx.Task.Configuration[BindingKey+key] = string(val)
		} else {
			for uKey, uVal := range unmarshalled {
				ctx.Task.Configuration[BindingKey+uKey] = uVal
			}
		}
	}
}

func (a *CommandActions) CreateSecretCpSvcKey(ctx *service.ActionContext, ns, secretName, cpSvcKey string) error {

	clientset, err := ctx.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "cannot get a target cluster client set")
	}

	repo := secrets.NewSecretRepo(ns, clientset)
	return repo.SecretCpSvcKey(
		context.Background(),
		secretName,
		cpSvcKey,
	)

}

func (a *CommandActions) CreateSecretTLS(ctx *service.ActionContext, ns, secretName string) (map[string][]byte, error) {

	clientset, err := ctx.KubeClient.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get a target cluster client set")
	}

	data, err := ssl.GenerateCertificate()
	if err != nil {
		return nil, err
	}

	repo := secrets.NewSecretRepo(ns, clientset)
	return repo.SaveSecretTLS(
		context.Background(),
		secretName,
		data[0],
		data[1],
	)
}

func (a *CommandActions) CreateCARootSecret(ctx *service.ActionContext, caClient connectivityclient.ConnectivityClient) error {

	ca, err := caClient.GetCA()

	if err != nil {
		return err
	}

	clientset, err := ctx.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "cannot get a target cluster client set")
	}

	if err := makeIstioCASecret(ctx.Task, clientset, ca); err != nil {
		return errors.Wrap(err, "cannot create Istio CA secret")
	}

	return nil
}

func (a *CommandActions) Remove(ctx *service.ActionContext) error {
	component := chart.NewComponentBuilder(ctx.Task.Version, ctx.Task.Component).
		WithNamespace(ctx.Task.Namespace).
		WithProfile(ctx.Task.Profile).
		WithConfiguration(ctx.Task.Configuration).
		WithURL(ctx.Task.URL).
		Build()

	authenticator, err := rendering.NewExternalComponentAuthenticator()
	if err != nil {
		return errors.Wrap(err, "failed to create chart provider")
	}
	component.SetExternalComponentAuthentication(authenticator)

	manifest, err := ctx.ChartProvider.RenderManifest(component)
	if err != nil {
		return errors.Wrap(err, "Error during rendering manifest for removal")
	}

	_, err = ctx.KubeClient.Delete(ctx.Context, manifest.Manifest, ctx.Task.Namespace)
	if err != nil {
		return errors.Wrap(err, "Error during removal")
	}

	return a.removeSecrets(ctx)
}

func (a *CommandActions) removeSecrets(ctx *service.ActionContext) error {

	for _, item := range [][2]string{
		{"cc-certs", "istio-system"},
		{"cc-certs-cacert", "istio-system"},
		{smSecretName, kymaSystem},
	} {
		_, err := ctx.KubeClient.DeleteResource(ctx.Context, "secret", item[0], item[1])
		if err != nil {
			msg := fmt.Sprintf("Error during removal of %s in %s", item[0], item[1])
			ctx.Logger.Error(msg)
			return errors.Wrap(err, "Error during removal of cc-certs in istio-system")
		}
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
