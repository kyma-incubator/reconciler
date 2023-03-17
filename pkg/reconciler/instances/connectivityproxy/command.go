package connectivityproxy

import (
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/rendering"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apiCoreV1 "k8s.io/api/core/v1"
	k8s "k8s.io/client-go/kubernetes"
)

const (
	BindingKey             = "global.binding."
	SkipManifestAnnotation = "reconciler.kyma-project.io/skip-rendering-on-upgrade"
)

//go:generate mockery --name=Commands --output=mocks --outpkg=connectivityproxymocks --case=underscore
type Commands interface {
	InstallOrUpgrade(*service.ActionContext, *appsv1.StatefulSet, *apiCoreV1.Secret) error
	CopyResources(*service.ActionContext) error
	Remove(*service.ActionContext) error
	PopulateConfigs(*service.ActionContext, *apiCoreV1.Secret)
}

type CommandActions struct {
	install     service.Operation
	copyFactory []CopyFactory
}

func (a *CommandActions) InstallOrUpgrade(context *service.ActionContext, app *appsv1.StatefulSet, credSecret *apiCoreV1.Secret) error {

	chartProvider, err := a.getChartProvider(context, app, credSecret)

	if err != nil {
		return errors.Wrap(err, "failed to create chart provider")
	}

	err = a.install.Invoke(context.Context, chartProvider, context.Task, context.KubeClient)
	if err != nil {
		return errors.Wrap(err, "failed to invoke installation")
	}

	return nil
}

func (a *CommandActions) getChartProvider(context *service.ActionContext, app *appsv1.StatefulSet, credSecret *apiCoreV1.Secret) (chart.Provider, error) {
	authenticator, err := rendering.NewExternalComponentAuthenticator()
	if err != nil {
		return nil, err
	}
	chartProviderWithAuthentication := rendering.NewProviderWithAuthentication(context.ChartProvider, authenticator)

	filterOutManifests := rendering.NewFilterOutAnnotatedManifests(SkipManifestAnnotation)
	filters := []rendering.FilterFunc{filterOutManifests}
	return rendering.NewProviderWithFilters(chartProviderWithAuthentication, filters...), nil

	return chartProviderWithAuthentication, nil
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

	caClient, err := NewConnectivityCAClient(context.Task)

	if err != nil {
		return errors.Wrap(err, "cannot create Connectivity CA client")
	}

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
	configs := task.Configuration

	istioSecretName, ok := configs["istio.secret.name"]

	if !ok {
		return errors.New("missing configuration value istio.secret.name")
	}

	istioNamespace, ok := configs["istio.secret.namespace"]
	if !ok || istioNamespace == nil || istioNamespace == "" {
		istioNamespace = "istio-system"
	}
	istioSecretKey, ok := configs["istio.secret.key"]
	if !ok || istioSecretKey == nil || istioSecretKey == "" {
		istioSecretKey = "cacert"
	}

	strNamespace := fmt.Sprintf("%v", istioNamespace)
	strSecretKey := fmt.Sprintf("%v", istioSecretKey)
	strSecretName := fmt.Sprintf("%v", istioSecretName)

	repo := NewSecretRepo(strNamespace, targetClientSet)

	return repo.SaveIstioCASecret(strSecretName, strSecretKey, ca)
}

//func istioCASecretCreate(task *reconciler.Task, targetClientSet k8s.Interface) *SecretCopy {
//	configs := task.Configuration
//
//	istioNamespace := configs["istio.secret.namespace"]
//	if istioNamespace == nil || istioNamespace == "" {
//		istioNamespace = "istio-system"
//	}
//	istioSecretKey := configs["istio.secret.key"]
//	if istioSecretKey == nil || istioSecretKey == "" {
//		istioSecretKey = "cacert"
//	}
//
//	return &SecretCopy{
//		Namespace:       fmt.Sprintf("%v", istioNamespace),
//		Name:            fmt.Sprintf("%v", configs["istio.secret.name"]),
//		targetClientSet: targetClientSet,
//		from: &FromURL{
//			URL: fmt.Sprintf("%v%v",
//				configs["global.binding.url"],
//				configs["global.binding.CAs_path"]),
//			Key: fmt.Sprintf("%v", istioSecretKey),
//		},
//	}
//}
