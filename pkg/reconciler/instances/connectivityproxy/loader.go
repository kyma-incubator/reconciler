package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	apiCoreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//go:generate mockery --name=Loader --output=mocks --outpkg=connectivityproxymocks --case=underscore
type Loader interface {
	FindBindingOperator(context *service.ActionContext) (*unstructured.Unstructured, error)
	FindBindingCatalog(context *service.ActionContext) (*unstructured.Unstructured, error)
	FindSecret(*service.ActionContext, *unstructured.Unstructured) (*apiCoreV1.Secret, error)
}

type K8sLoader struct {
	Search Search
}

func (a *K8sLoader) FindBindingOperator(context *service.ActionContext) (*unstructured.Unstructured, error) {
	search := Search{}
	return search.findByCriteria([]Locator{
		{
			resource:       "serviceinstance",
			field:          "spec.serviceOfferingName",
			client:         context.KubeClient,
			searchNextBy:   "metadata.name",
			referenceValue: "connectivity",
		},
		{
			resource:     "servicebinding",
			field:        "spec.serviceInstanceName",
			client:       context.KubeClient,
			searchNextBy: "spec.secretName",
		},
	})
}

func (a *K8sLoader) FindBindingCatalog(context *service.ActionContext) (*unstructured.Unstructured, error) {
	search := Search{}
	return search.findByCriteria([]Locator{
		{
			resource:       "serviceinstance",
			field:          "spec.clusterServiceClassExternalName",
			client:         context.KubeClient,
			searchNextBy:   "metadata.name",
			referenceValue: "connectivity",
		},
		{
			resource:     "servicebinding",
			field:        "spec.instanceRef.name",
			client:       context.KubeClient,
			searchNextBy: "spec.secretName",
		},
	})
}

func (a *K8sLoader) FindSecret(context *service.ActionContext, binding *unstructured.Unstructured) (*apiCoreV1.Secret, error) {
	var bindingUns Map = binding.Object

	name, err := bindingUns.getSecretName()
	if err != nil {
		return nil, errors.Wrap(err, "Error while extracting secret")
	}

	namespace := context.Task.Namespace
	if namespace == "" {
		context.Logger.Info("No namespace set, assuming default.")
		namespace = "default"
	}

	return context.KubeClient.GetSecret(context.Context, name, namespace)
}
