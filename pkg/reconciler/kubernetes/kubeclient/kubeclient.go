// solution from https://github.com/billiford/go-clouddriver/blob/master/pkg/kubernetes/client.go

package kubeclient

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/util"
)

type Metadata struct {
	Name      string
	Namespace string
	Group     string
	Version   string
	Resource  string
	Kind      string
}

type KubeClient struct {
	dynamicClient dynamic.Interface
	config        *rest.Config
	mapper        *restmapper.DeferredDiscoveryRESTMapper
}

func NewKubeClient(kubeconfig string) (*KubeClient, error) {
	config, err := getRestConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	mapper, err := getDiscoveryMapper(config)
	if err != nil {
		return nil, err
	}

	return &KubeClient{
		dynamicClient: dynamicClient,
		config:        config,
		mapper:        mapper,
	}, nil
}

func (kube *KubeClient) Apply(u *unstructured.Unstructured) (*k8s.Resource, error) {
	return kube.ApplyWithNamespaceOverride(u, "")
}

// ApplyWithNamespaceOverride applies a given manifest with an optional namespace to override.
// If no namespace is set on the manifest and no namespace override is passed in then we set the namespace to 'default'.
// If namespaceOverride is empty it will NOT override the namespace set on the manifest.
// We only override the namespace if the manifest is NOT cluster scoped (i.e. a ClusterRole) and namespaceOverride is NOT an
// empty string.
func (kube *KubeClient) ApplyWithNamespaceOverride(u *unstructured.Unstructured, namespaceOverride string) (*k8s.Resource, error) {
	metadata := &k8s.Resource{}
	gvk := u.GroupVersionKind()

	restMapping, err := kube.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return metadata, err
	}

	gv := gvk.GroupVersion()
	kube.config.GroupVersion = &gv

	restClient, err := newRestClient(*kube.config, gv)
	if err != nil {
		return metadata, err
	}

	helper := resource.NewHelper(restClient, restMapping)

	setDefaultNamespaceIfScopedAndNoneSet(namespaceOverride, u, helper)

	info := &resource.Info{
		Client:          restClient,
		Mapping:         restMapping,
		Namespace:       u.GetNamespace(),
		Name:            u.GetName(),
		Source:          "",
		Object:          u,
		ResourceVersion: restMapping.Resource.Version,
	}

	patcher := newPatcher(info, helper)

	// Get the modified configuration of the object. Embed the result
	// as an annotation in the modified configuration, so that it will appear
	// in the patch sent to the server.
	modified, err := util.GetModifiedConfiguration(info.Object, true, unstructured.UnstructuredJSONScheme)
	if err != nil {
		return metadata, err
	}

	if err := info.Get(); err != nil {
		if !k8serrors.IsNotFound(err) {
			return metadata, err
		}

		// Create the resource if it doesn't exist
		// First, update the annotation used by kubectl kubeClient
		if err := util.CreateApplyAnnotation(info.Object, unstructured.UnstructuredJSONScheme); err != nil {
			return metadata, err
		}

		// Then create the resource and skip the three-way merge
		obj, err := helper.Create(info.Namespace, true, info.Object)
		if err != nil {
			return metadata, err
		}

		_ = info.Refresh(obj, true)
	}

	_, patchedObject, err := patcher.Patch(info.Object, modified, info.Namespace, info.Name)
	if err != nil {
		return metadata, err
	}

	_ = info.Refresh(patchedObject, true)

	metadata.Name = u.GetName()
	metadata.Namespace = u.GetNamespace()
	metadata.Kind = gvk.Kind

	return metadata, nil
}

func (kube *KubeClient) GetClientSet() (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(kube.config)
}

func (kube *KubeClient) DeleteResourceByKindAndNameAndNamespace(kind, name, namespace string, do metav1.DeleteOptions) (*k8s.Resource, error) {
	gvk, err := kube.mapper.KindFor(schema.GroupVersionResource{
		Resource: kind,
	})
	if err != nil {
		return nil, err
	}

	var isNamespaceResource = strings.ToLower(gvk.Kind) == "namespace"

	if !isNamespaceResource && namespace == "" { //set qualified namespace (except resource is of kind 'namespace')
		namespace = "default"
	}

	restMapping, err := kube.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	restClient, err := newRestClient(*kube.config, gvk.GroupVersion())
	if err != nil {
		return nil, err
	}

	helper := resource.NewHelper(restClient, restMapping)

	if helper.NamespaceScoped {
		err = kube.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Delete(context.TODO(), name, do)
	} else {
		err = kube.dynamicClient.
			Resource(restMapping.Resource).
			Delete(context.TODO(), name, do)
	}

	//return deleted resource
	if isNamespaceResource {
		namespace = "" //namespace resources have always an empty namespace field
	}
	return &k8s.Resource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}, err
}

// Get a manifest by resource/kind (example: 'pods' or 'pod'),
// name (example: 'my-pod'), and namespace (example: 'my-namespace').
func (kube *KubeClient) Get(kind, name, namespace string) (*unstructured.Unstructured, error) {
	gvk, err := kube.mapper.KindFor(schema.GroupVersionResource{Resource: kind})
	if err != nil {
		return nil, err
	}

	restMapping, err := kube.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	restClient, err := newRestClient(*kube.config, gvk.GroupVersion())
	if err != nil {
		return nil, err
	}

	var u *unstructured.Unstructured

	helper := resource.NewHelper(restClient, restMapping)
	if helper.NamespaceScoped {
		u, err = kube.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		u, err = kube.dynamicClient.
			Resource(restMapping.Resource).
			Get(context.TODO(), name, metav1.GetOptions{})
	}

	return u, err
}

// ListResource lists all resources by their kind or resource (e.g. "replicaset" or "replicasets").
func (kube *KubeClient) ListResource(resource string, lo metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	gvr, err := kube.mapper.ResourceFor(schema.GroupVersionResource{Resource: resource})
	if err != nil {
		return nil, err
	}
	return kube.dynamicClient.Resource(gvr).List(context.TODO(), lo)
}

func (kube *KubeClient) Patch(kind, name, namespace string, p []byte) (Metadata, *unstructured.Unstructured, error) {
	return kube.PatchUsingStrategy(kind, name, namespace, p, types.StrategicMergePatchType)
}

func (kube *KubeClient) PatchUsingStrategy(kind, name, namespace string, p []byte, strategy types.PatchType) (Metadata, *unstructured.Unstructured, error) {
	metadata := Metadata{}
	gvk, err := kube.mapper.KindFor(schema.GroupVersionResource{Resource: kind})
	if err != nil {
		return metadata, nil, err
	}

	restMapping, err := kube.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return metadata, nil, err
	}

	restClient, err := newRestClient(*kube.config, gvk.GroupVersion())
	if err != nil {
		return metadata, nil, err
	}

	helper := resource.NewHelper(restClient, restMapping)

	var u *unstructured.Unstructured

	if helper.NamespaceScoped {
		u, err = kube.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Patch(context.TODO(), name, strategy, p, metav1.PatchOptions{})
	} else {
		u, err = kube.dynamicClient.
			Resource(restMapping.Resource).
			Patch(context.TODO(), name, strategy, p, metav1.PatchOptions{})
	}

	if err != nil {
		return metadata, nil, err
	}

	gvr := restMapping.Resource

	metadata.Name = u.GetName()
	metadata.Namespace = u.GetNamespace()
	metadata.Group = gvr.Group
	metadata.Resource = gvr.Resource
	metadata.Version = gvr.Version
	metadata.Kind = gvk.Kind

	return metadata, u, err
}

func newRestClient(restConfig rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	restConfig.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	restConfig.GroupVersion = &gv

	if len(gv.Group) == 0 {
		restConfig.APIPath = "/api"
	} else {
		restConfig.APIPath = "/apis"
	}

	return rest.RESTClientFor(&restConfig)
}

func getDiscoveryMapper(restConfig *rest.Config) (*restmapper.DeferredDiscoveryRESTMapper, error) {
	// Prepare a RESTMapper to find GVR
	dc, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create new discovery client")
	}

	discoveryMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	return discoveryMapper, nil
}

func getRestConfig(kubeconfig string) (*rest.Config, error) {
	return clientcmd.BuildConfigFromKubeconfigGetter("", func() (config *clientcmdapi.Config, e error) {
		return clientcmd.Load([]byte(kubeconfig))
	})
}

func setDefaultNamespaceIfScopedAndNoneSet(namespace string, u *unstructured.Unstructured, helper *resource.Helper) {
	if helper.NamespaceScoped {
		resNamespace := u.GetNamespace()
		if resNamespace == "" {
			if namespace == "" {
				namespace = "default"
			}
			u.SetNamespace(namespace)
		}
	}
}
