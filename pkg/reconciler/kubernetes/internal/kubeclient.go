// solution from https://github.com/billiford/go-clouddriver/blob/master/pkg/kubernetes/client.go

package internal

import (
	"bytes"
	"context"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/kube"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"strings"

	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
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
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// Metadata is an internal type to transfer data to the adapter
type Metadata struct {
	Name      string
	Namespace string
	Resource  string
	Group     string
	Version   string
	Kind      string
}

type KubeClient struct {
	dynamicClient dynamic.Interface
	config        *rest.Config
	mapper        *restmapper.DeferredDiscoveryRESTMapper
	helmClient    *kube.Client
}

func NewKubeClient(kubeconfig string, logger *zap.SugaredLogger) (*KubeClient, error) {
	config, err := getRestConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	config.WarningHandler = &loggingWarningHandler{logger: logger}
	return newForConfig(config)
}

func NewInClusterClient(logger *zap.SugaredLogger) (*KubeClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	config.WarningHandler = &loggingWarningHandler{logger: logger}
	return newForConfig(config)
}

func newForConfig(config *rest.Config) (*KubeClient, error) {
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
		helmClient:    kube.New(NewRESTClientGetter(config)),
	}, nil
}

func (k *KubeClient) Apply(u *unstructured.Unstructured) (*Metadata, error) {
	return k.ApplyWithNamespaceOverride(u, "")
}

// ApplyWithNamespaceOverride applies a given manifest with an optional namespace to override.
// If no namespace is set on the manifest and no namespace override is passed in then we set the namespace to 'default'.
// If namespaceOverride is empty it will NOT override the namespace set on the manifest.
// We only override the namespace if the manifest is NOT cluster scoped (i.e. a ClusterRole) and namespaceOverride is NOT an
// empty string.
func (k *KubeClient) ApplyWithNamespaceOverride(u *unstructured.Unstructured, namespaceOverride string) (*Metadata, error) {
	gvk := u.GroupVersionKind()
	metadata := &Metadata{
		Kind: gvk.Kind,
		Name: u.GetName(),
	}

	restMapping, err := k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	gv := gvk.GroupVersion()
	k.config.GroupVersion = &gv

	restClient, err := newRestClient(*k.config, gv)
	if err != nil {
		return nil, err
	}

	helper := resource.NewHelper(restClient, restMapping)

	setDefaultNamespaceIfScopedAndNoneSet(namespaceOverride, u, helper)
	setNamespaceIfScoped(namespaceOverride, u, helper)
	metadata.Namespace = u.GetNamespace()

	updateStrategyResolver := newDefaultUpdateStrategyResolver(helper)
	strategy, err := updateStrategyResolver.Resolve(u)
	if err != nil {
		return nil, err
	}

	if strategy == SkipUpdateStrategy {
		return metadata, nil
	}

	manifest, err := yaml.Marshal(u.Object)
	if err != nil {
		return nil, err
	}
	target, err := k.helmClient.Build(bytes.NewBuffer(manifest), false)
	if err != nil {
		return nil, err
	}

	originalInfo := &resource.Info{
		Client:          restClient,
		Mapping:         restMapping,
		Namespace:       u.GetNamespace(),
		Name:            u.GetName(),
		Source:          "",
		ResourceVersion: restMapping.Resource.Version,
	}

	var original kube.ResourceList
	if originalInfo.Get() == nil {
		original = kube.ResourceList{
			originalInfo,
		}
	}

	if err != nil && !k8serr.IsNotFound(err) {
		return nil, err
	}

	replaceResource := strategy == ReplaceUpdateStrategy
	if _, err := k.helmClient.Update(original, target, replaceResource); err != nil {
		return nil, err
	}

	return metadata, nil
}

func (k *KubeClient) GetClientSet() (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(k.config)
}

func (k *KubeClient) DeleteResourceByKindAndNameAndNamespace(kind, name, namespace string, do metav1.DeleteOptions) (*Metadata, error) {
	gvk, err := k.mapper.KindFor(schema.GroupVersionResource{
		Resource: kind,
	})
	if err != nil {
		return nil, err
	}

	var isNamespaceResource = strings.ToLower(gvk.Kind) == "namespace"

	if !isNamespaceResource && namespace == "" { //set qualified namespace (except resource is of kind 'namespace')
		namespace = "default"
	}

	restMapping, err := k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	restClient, err := newRestClient(*k.config, gvk.GroupVersion())
	if err != nil {
		return nil, err
	}

	helper := resource.NewHelper(restClient, restMapping)

	if helper.NamespaceScoped {
		err = k.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Delete(context.TODO(), name, do)
	} else {
		err = k.dynamicClient.
			Resource(restMapping.Resource).
			Delete(context.TODO(), name, do)
	}

	//return deleted resource
	if isNamespaceResource {
		namespace = "" //namespace resources have always an empty namespace field
	}
	return &Metadata{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}, err
}

// Get a manifest by resource/kind (example: 'pods' or 'pod'),
// name (example: 'my-pod'), and namespace (example: 'my-namespace').
func (k *KubeClient) Get(kind, name, namespace string) (*unstructured.Unstructured, error) {
	gvk, err := k.mapper.KindFor(schema.GroupVersionResource{Resource: kind})
	if err != nil {
		return nil, err
	}

	restMapping, err := k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	restClient, err := newRestClient(*k.config, gvk.GroupVersion())
	if err != nil {
		return nil, err
	}

	var u *unstructured.Unstructured

	helper := resource.NewHelper(restClient, restMapping)
	if helper.NamespaceScoped {
		u, err = k.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		u, err = k.dynamicClient.
			Resource(restMapping.Resource).
			Get(context.TODO(), name, metav1.GetOptions{})
	}

	return u, err
}

// ListResource lists all resources by their kind or resource (e.g. "replicaset" or "replicasets").
func (k *KubeClient) ListResource(resource string, lo metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	gvr, err := k.mapper.ResourceFor(schema.GroupVersionResource{Resource: resource})
	if err != nil {
		return nil, err
	}
	return k.dynamicClient.Resource(gvr).List(context.TODO(), lo)
}

func (k *KubeClient) Patch(kind, name, namespace string, p []byte) (*Metadata, *unstructured.Unstructured, error) {
	return k.PatchUsingStrategy(kind, name, namespace, p, types.StrategicMergePatchType)
}

func (k *KubeClient) PatchUsingStrategy(kind, name, namespace string, p []byte, strategy types.PatchType) (*Metadata, *unstructured.Unstructured, error) {
	metadata := &Metadata{}
	gvk, err := k.mapper.KindFor(schema.GroupVersionResource{Resource: kind})
	if err != nil {
		return metadata, nil, err
	}

	restMapping, err := k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return metadata, nil, err
	}

	restClient, err := newRestClient(*k.config, gvk.GroupVersion())
	if err != nil {
		return metadata, nil, err
	}

	helper := resource.NewHelper(restClient, restMapping)

	var u *unstructured.Unstructured

	if helper.NamespaceScoped {
		u, err = k.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Patch(context.TODO(), name, strategy, p, metav1.PatchOptions{})
	} else {
		u, err = k.dynamicClient.
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

func (k *KubeClient) DeleteNamespace(namespace string) error {
	getter := NewRESTClientGetter(k.config)
	factory := cmdutil.NewFactory(getter)
	r := factory.NewBuilder().
		Unstructured().
		NamespaceParam(namespace).DefaultNamespace().
		LabelSelectorParam("").
		FieldSelectorParam("").
		RequestChunksOf(500).
		ResourceTypeOrNameArgs(true, "all").
		ContinueOnError().
		Latest().
		Flatten().
		Do()
	infos, err := r.Infos()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		namespaceRes := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
		err = k.dynamicClient.
			Resource(namespaceRes).
			Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	}
	return err
}

func (k *KubeClient) GetHost() string {
	if k.config == nil {
		return ""
	}

	return k.config.Host
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

func setNamespaceIfScoped(namespace string, u *unstructured.Unstructured, helper *resource.Helper) {
	if u.GetNamespace() == "" && helper.NamespaceScoped {
		u.SetNamespace(namespace)
	}
}
