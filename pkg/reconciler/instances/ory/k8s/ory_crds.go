package k8s

import (
	"context"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apixv1beta1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	k8sRetry "k8s.io/client-go/util/retry"
)

type OryFinalizersHandler interface {
	FindAndDeleteOryFinalizers(kubeconfigData string, logger *zap.SugaredLogger) error
}

type DefaultOryFinalizersHandler struct {
	apixClient apixv1beta1client.ApiextensionsV1beta1Interface
	dynamic    dynamic.Interface
	logger     *zap.SugaredLogger
}

func NewDefaultOryFinalizersHandler() *DefaultOryFinalizersHandler {
	return &DefaultOryFinalizersHandler{}
}

func (h *DefaultOryFinalizersHandler) FindAndDeleteOryFinalizers(kubeconfigData string, logger *zap.SugaredLogger) error {
	h.logger = logger
	var res []schema.GroupVersionResource

	config, err := restConfig(kubeconfigData)
	if err != nil {
		return err
	}

	if h.apixClient, err = apixv1beta1client.NewForConfig(config); err != nil {
		return err
	}
	h.dynamic, err = dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	crds, err := h.apixClient.CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}

	if crds == nil {
		return nil
	}

	for _, crd := range crds.Items {
		crdef := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  crd.Spec.Version,
			Resource: crd.Spec.Names.Plural,
		}
		if crd.Kind == "OAuth2Client" {
			res = append(res, crdef)
		}
	}

	var lastErr error
	for _, crdef := range res {
		err := h.removeFinalizersFromAllInstancesOf(crdef) //Continue in case of an error
		if err != nil {
			lastErr = err
			h.logger.Errorf("Error while dropping finalizers for CustomResourceDefinition \"%s\": %s", crdef.String(), err.Error())
		}
	}

	return lastErr //return last error (if any)
}

func (h *DefaultOryFinalizersHandler) removeFinalizersFromAllInstancesOf(crdef schema.GroupVersionResource) error {
	h.logger.Debugf("Dropping finalizers for all ory custom resources of type: %s.%s/%s", crdef.Resource, crdef.Group, crdef.Version)
	defer h.logger.Debugf("Finished dropping finalizers for ory custom resources of type: %s.%s/%s", crdef.Resource, crdef.Group, crdef.Version)

	customResourceList, err := h.dynamic.Resource(crdef).Namespace(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}

	if customResourceList == nil {
		return nil
	}

	for i := range customResourceList.Items {
		instance := customResourceList.Items[i]
		retryErr := k8sRetry.RetryOnConflict(k8sRetry.DefaultRetry, func() error { return h.removeCustomResourceFinalizers(crdef, instance) })
		if retryErr != nil {
			return errors.Wrapf(retryErr, "deleting ory finalizer for %s.%s/%s \"%s\" failed", crdef.Resource, crdef.Group, crdef.Version, instance.GetName())
		}
	}

	return nil
}

func (h *DefaultOryFinalizersHandler) removeCustomResourceFinalizers(crdef schema.GroupVersionResource, instance unstructured.Unstructured) error {
	// Retrieve the latest version of Custom Resource before attempting update
	// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
	res, err := h.dynamic.Resource(crdef).Namespace(instance.GetNamespace()).Get(context.Background(), instance.GetName(), metav1.GetOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}
	if res == nil {
		return nil
	}

	if len(res.GetFinalizers()) > 0 {
		h.logger.Debugf("Found finalizers for \"%s\" %s, deleting", res.GetName(), instance.GetKind())

		res.SetFinalizers(nil)
		_, err := h.dynamic.Resource(crdef).Namespace(res.GetNamespace()).Update(context.Background(), res, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		h.logger.Debugf("Deleted finalizer for \"%s\" %s", res.GetName(), instance.GetKind())
	}

	return nil
}

// restConfig loads the rest configuration needed by k8s clients to interact with clusters based on the kubeconfig.
// Loading rules are based on standard defined kubernetes config loading.
func restConfig(kubeconfigData string) (*rest.Config, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfigData))
	if err != nil {
		return nil, err
	}
	cfg.WarningHandler = rest.NoWarnings{}
	return cfg, nil
}
