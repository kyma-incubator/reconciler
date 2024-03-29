package connectivityproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/configmaps"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/connectivityclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/rendering"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/secrets"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/ssl"
	"github.com/pkg/errors"
	apiCoreV1 "k8s.io/api/core/v1"
	errk8s "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	CreateSecretMappingOperator(*service.ActionContext, string) ([]byte, error)
	CreateSecretCpSvcKey(ctx *service.ActionContext, ns, secretName, cpSvcKey string) error
	Remove(*service.ActionContext) error
	CreateServiceMappingConfigMap(ctx *service.ActionContext, ns, configMapName string) error
	FixConfiguration(ctx *service.ActionContext, ns, name, tunnelURL string) error
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

func populateConfigs(configuration map[string]interface{}, bindingSecret *apiCoreV1.Secret) {
	for key, val := range bindingSecret.Data {
		flatValues(configuration, key, val)
	}
}

func flatValues(configuration map[string]interface{}, key string, value []byte) {

	var unmarshalled map[string]interface{}

	if err := json.Unmarshal(value, &unmarshalled); err != nil {
		configuration[BindingKey+key] = string(value)
		return
	}

	for uKey, uVal := range unmarshalled {
		strVal, ok := uVal.(string)
		if ok {
			flatValues(configuration, uKey, []byte(strVal))
			continue
		}
		configuration[BindingKey+uKey] = uVal

	}
}

func (a *CommandActions) CreateSecretMappingOperator(s *service.ActionContext, ns string) ([]byte, error) {
	cs, err := s.KubeClient.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get a target cluster client set")
	}

	repo := secrets.NewSecretRepo(ns, cs)
	data, err := ssl.GenerateCertificate(
		"connectivity-proxy-smv.kyma-system.svc",
		[]string{"connectivity-proxy-smv.kyma-system.svc"},
	)
	if err != nil {
		return nil, err
	}

	secretData, err := repo.SaveSecretMappingOperator(
		s.Context,
		mappingOperatorSecretName,
		data[0],
		data[1],
	)

	if err != nil {
		return nil, err
	}

	caData, found := secretData[secrets.TagTLSCa]
	if !found {
		return nil, fmt.Errorf("not found: %s in %s/%s", secrets.TagTLSCa, kymaSystem, smSecretName)
	}

	return caData, nil
}

func (a *CommandActions) CreateSecretCpSvcKey(ctx *service.ActionContext, ns, secretName, cpSvcKey string) error {
	clientset, err := ctx.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "cannot get a target cluster client set")
	}

	repo := secrets.NewSecretRepo(ns, clientset)
	return repo.SaveSecretCpSvcKey(
		ctx.Context,
		secretName,
		cpSvcKey,
	)
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

	if err := makeIstioCASecret(context.Context, context.Task, clientset, ca); err != nil {
		return errors.Wrap(err, "cannot create Istio CA secret")
	}

	return nil
}

func (a *CommandActions) preRemoveCheck(context *service.ActionContext) error {
	// make sure mappings are installed
	crds, err := context.KubeClient.ListResource(context.Context, "customresourcedefinitions", metav1.ListOptions{
		FieldSelector: "metadata.name=servicemappings.connectivityproxy.sap.com",
	})
	if err != nil {
		return err
	}
	// servicemappings are not available on a cluster
	if len(crds.Items) == 0 {
		return nil
	}
	// check if any servicemapping CR is available on a cluster
	mappings, err := context.KubeClient.ListResource(context.Context, "servicemappings", metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(mappings.Items) > 0 {
		return fmt.Errorf("%w: unable to delete connectivity-proxy, servicemappings detected", ErrReconciliationAborted)
	}
	return nil
}

func (a *CommandActions) Remove(context *service.ActionContext) error {
	if err := a.preRemoveCheck(context); err != nil {
		return err
	}

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

	return removeCpResources(context)
}

func aggregateErrors(errs []error) error {
	var messages []string

	for _, err := range errs {
		messages = append(messages, err.Error())
	}

	if len(messages) == 0 {
		return nil
	}

	aggMessage := strings.Join(messages, "; ")
	return errors.New(aggMessage)
}

type resourceType string

const (
	rscTypeSecret    = "secret"
	rscTypeConfigmap = "configmap"
)

func removeResource(context *service.ActionContext, t resourceType, name, ns string) error {
	_, err := context.KubeClient.DeleteResource(context.Context, string(t), name, ns)
	if err != nil && !errk8s.IsNotFound(err) {
		errMsg := fmt.Sprintf("Error during removal of %s in %s", name, ns)
		context.Logger.Error(errMsg)
		return errors.Wrap(err, errMsg)
	}

	return nil
}

func removeCpResources(context *service.ActionContext) error {
	var errs []error
	// remove secrets
	for _, rscInfo := range []struct {
		name string
		ns   string
		t    resourceType
	}{
		{name: "cc-certs", ns: "istio-system", t: rscTypeSecret},
		{name: "cc-certs-cacert", ns: "istio-system", t: rscTypeSecret},
		{name: mappingOperatorSecretName, ns: kymaSystem, t: rscTypeSecret},
		{name: mappingsConfigMap, ns: kymaSystem, t: rscTypeConfigmap},
		{name: cpSvcKeySecretName, ns: kymaSystem, t: rscTypeSecret},
	} {
		if err := removeResource(context, rscInfo.t, rscInfo.name, rscInfo.ns); err != nil {
			errs = append(errs, err)
		}
	}

	return aggregateErrors(errs)
}

func makeIstioCASecret(ctx context.Context, task *reconciler.Task, targetClientSet k8s.Interface, ca []byte) error {

	namespace, secretKey, secretName, err := getIstioSecretCfg(task.Configuration)

	if err != nil {
		return err
	}

	repo := secrets.NewSecretRepo(namespace, targetClientSet)
	return repo.SaveIstioCASecret(ctx, secretName, secretKey, ca)
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

func (a *CommandActions) CreateServiceMappingConfigMap(svcActionCtx *service.ActionContext, ns, configMapName string) error {
	clientset, err := svcActionCtx.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "cannot get a target cluster client set")
	}

	return configmaps.NewConfigMapRepo(ns, clientset).CreateServiceMappingConfig(svcActionCtx.Context, configMapName)
}

// FixConfiguration function corrects the config map to make sure certificate domain, and Connectivity Proxy's configuration match.
// This is 2.9.2 specific.
func (a *CommandActions) FixConfiguration(ctx *service.ActionContext, ns, name, tunnelURL string) error {
	clientset, err := ctx.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "cannot get a target cluster client set")
	}

	return configmaps.NewConfigMapRepo(ns, clientset).FixConfiguration(ns, name, tunnelURL)
}
