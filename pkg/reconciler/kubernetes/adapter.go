package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"time"

	batchv1 "k8s.io/api/batch/v1"

	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultNamespace  = "default"
	namespaceManifest = `
apiVersion: v1
kind: Namespace
metadata:
  name: ""`
	progressTrackerInterval   = 5 * time.Second
	progressTrackerTimeout    = 2 * time.Minute
	progressTrackerMaxRetries = 10
	progressTrackerRetryDelay = 1 * time.Second
)

type kubeClientAdapter struct {
	kubeconfig    string
	logger        *zap.SugaredLogger
	config        *Config
	restConfig    *rest.Config
	mapper        *restmapper.DeferredDiscoveryRESTMapper
	helmClient    *kube.Client
	dynamicClient dynamic.Interface
}

type Config struct {
	ProgressInterval time.Duration
	ProgressTimeout  time.Duration
	MaxRetries       int
	RetryDelay       time.Duration
}

type Metadata struct {
	Name      string
	Namespace string
	Resource  string
	Group     string
	Version   string
	Kind      string
}

func NewKubernetesClient(kubeconfig string, logger *zap.SugaredLogger, config *Config) (Client, error) {
	if config == nil {
		config = &Config{
			ProgressInterval: progressTrackerInterval,
			ProgressTimeout:  progressTrackerTimeout,
			MaxRetries:       progressTrackerMaxRetries,
			RetryDelay:       progressTrackerRetryDelay,
		}
	}

	restConfig, err := getRestConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	mapper, err := getDiscoveryMapper(restConfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return adapt(kubeconfig, logger, config, restConfig, mapper, dynamicClient), err
}

func NewInClusterClientSet(logger *zap.SugaredLogger) (kubernetes.Interface, error) {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Infof("Not able to create an In Cluster Client")
		return nil, nil
	}
	return kubernetes.NewForConfig(inClusterConfig)
}

func adapt(kubeconfig string, logger *zap.SugaredLogger, config *Config, restConfig *rest.Config, mapper *restmapper.DeferredDiscoveryRESTMapper, dynamicClient dynamic.Interface) *kubeClientAdapter {
	return &kubeClientAdapter{
		kubeconfig:    kubeconfig,
		logger:        logger,
		config:        config,
		restConfig:    restConfig,
		mapper:        mapper,
		dynamicClient: dynamicClient,
		helmClient:    kube.New(NewRESTClientGetter(restConfig)),
	}
}

func (g *kubeClientAdapter) Kubeconfig() string {
	return g.kubeconfig
}

func (g *kubeClientAdapter) PatchUsingStrategy(kind, name, namespace string, p []byte, strategy types.PatchType) error {
	metadata := &Metadata{}
	gvk, err := g.mapper.KindFor(schema.GroupVersionResource{Resource: kind})
	if err != nil {
		return err
	}

	restMapping, err := g.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	restClient, err := newRestClient(*g.restConfig, gvk.GroupVersion())
	if err != nil {
		return err
	}

	helper := resource.NewHelper(restClient, restMapping)

	var u *unstructured.Unstructured

	if helper.NamespaceScoped {
		u, err = g.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Patch(context.TODO(), name, strategy, p, metav1.PatchOptions{})
	} else {
		u, err = g.dynamicClient.
			Resource(restMapping.Resource).
			Patch(context.TODO(), name, strategy, p, metav1.PatchOptions{})
	}

	if err != nil {
		return err
	}

	gvr := restMapping.Resource

	metadata.Name = u.GetName()
	metadata.Namespace = u.GetNamespace()
	metadata.Group = gvr.Group
	metadata.Resource = gvr.Resource
	metadata.Version = gvr.Version
	metadata.Kind = gvk.Kind
	return nil
}

func (g *kubeClientAdapter) DeployByCompareWithOriginal(ctx context.Context, manifestOriginal, manifestTarget, namespace string, interceptors ...ResourceInterceptor) ([]*Resource, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	resourceInfoOriginal, err := g.helmClient.Build(bytes.NewBuffer([]byte(manifestOriginal)), false)
	if err != nil {
		g.logger.Errorf("Failed to process original manifest data for deploy: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestOriginal)
		return nil, err
	}

	resourceInfoTarget, err := g.convertToResourceInfoWithInterceptors(manifestTarget, namespace, interceptors)
	if err != nil {
		g.logger.Errorf("Failed to process target manifest data for deploy: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}

	deployedResources, err := g.deployResources(ctx, resourceInfoOriginal, resourceInfoTarget, namespace)

	if len(deployedResources) == 0 {
		g.logger.Warnf("Namespace '%s' was required for deploying the manifestTarget "+
			"but no resources were finally deployed into it", namespace)
	}

	// TODO: consider if make sense to delete resources which in resourceInfoOriginal but not in resourceInfoTarget.

	return deployedResources, err
}

func (g *kubeClientAdapter) Deploy(ctx context.Context, manifestTarget, namespace string, interceptors ...ResourceInterceptor) ([]*Resource, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	resourceInfoTarget, err := g.convertToResourceInfoWithInterceptors(manifestTarget, namespace, interceptors)
	if err != nil {
		g.logger.Errorf("Failed to process target manifest data for deploy: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}

	deployedResources, err := g.deployResources(ctx, resourceInfoTarget, resourceInfoTarget, namespace)

	if len(deployedResources) == 0 {
		g.logger.Warnf("Namespace '%s' was required for deploying the manifestTarget "+
			"but no resources were finally deployed into it", namespace)
	}

	return deployedResources, err
}

func (g *kubeClientAdapter) convertToResourceInfoWithInterceptors(manifestTarget string, namespace string, interceptors []ResourceInterceptor) ([]*resource.Info, error) {

	unstructsTarget, err := g.manifestToUnstructured(manifestTarget)
	if err != nil {
		return nil, err
	}

	unstructsTarget, err = g.addNamespaceUnstruct(unstructsTarget, namespace)
	if err != nil {
		return nil, err
	}
	//fill out the resourceListTarget map by kind
	resourceListTarget := NewResourceList(unstructsTarget)

	//apply interceptors to target
	for _, interceptor := range interceptors {
		if interceptor == nil {
			continue
		}

		err := interceptor.Intercept(resourceListTarget, namespace)
		if err != nil {
			g.logger.Errorf("One of the interceptors returned an error: %s", err)
			return nil, err
		}
	}

	resourceInfoTarget, err := g.convertToInfoList(resourceListTarget.resources)
	if err != nil {
		g.logger.Errorf("Failed to convert target unstructs data: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}
	return resourceInfoTarget, nil
}

func (g *kubeClientAdapter) deployResources(ctx context.Context, infoOriginalList kube.ResourceList, infoTargetList kube.ResourceList, namespace string) ([]*Resource, error) {
	pt, err := g.newProgressTracker()
	if err != nil {
		return nil, err
	}

	var deployedResources []*Resource
	for _, infoTarget := range infoTargetList {
		//Do intersect to make sure helmclient only do create/update but not delete resource which exists in original but not in target.
		intersectOriginal := kube.ResourceList{infoTarget}.Intersect(infoOriginalList)
		if len(intersectOriginal) == 0 {
			return nil, fmt.Errorf("could not find intersect between original and target resource")
		}
		deployedResource, err := g.deployResource(intersectOriginal[0], infoTarget, namespace)
		if err != nil {
			g.logger.Errorf("Failed to apply Kubernetes unstructured entity: %s", err)
			return nil, err
		}

		//add deploy resource to result
		g.logger.Debugf("Kubernetes deployedResource '%v' successfully deployed", deployedResource)
		deployedResources = append(deployedResources, deployedResource)

		//if resource is watchable, add it to progress tracker
		watchable, nonWatchableErr := progress.NewWatchableResource(deployedResource.Kind)
		if nonWatchableErr == nil { //add only watchable resources to progress tracker
			pt.AddResource(watchable, deployedResource.Namespace, deployedResource.Name)
		}
	}

	return deployedResources, pt.Watch(ctx, progress.ReadyState)
}

func (g *kubeClientAdapter) manifestToUnstructured(manifest string) ([]*unstructured.Unstructured, error) {
	unstructs, err := ToUnstructured([]byte(manifest), true)
	if err != nil {
		g.logger.Errorf("Failed to process manifest data to unstructured: %s", err)
		g.logger.Debugf("Manifest data: %s", manifest)
		return nil, err
	}
	return unstructs, nil
}

func (g *kubeClientAdapter) addWatchableResourceInfoToProgressTracker(info *resource.Info, pt *progress.Tracker) {
	resource := &Resource{
		Name:      info.Name,
		Kind:      info.Object.GetObjectKind().GroupVersionKind().Kind,
		Namespace: info.Namespace,
	}
	//add deploy resource to result
	g.logger.Debugf("Kubernetes resource '%v' successfully deployed", resource)
	//if resource is watchable, add it to progress tracker
	watchable, nonWatchableErr := progress.NewWatchableResource(resource.Kind)
	if nonWatchableErr == nil { //add only watchable resourceListTarget to progress tracker
		pt.AddResource(watchable, resource.Namespace, resource.Name)
	}
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

func (g *kubeClientAdapter) convertToInfoList(unstructs []*unstructured.Unstructured) ([]*resource.Info, error) {
	var resourceInfos []*resource.Info

	for _, unstruct := range unstructs {
		info, err := g.convertToInfo(unstruct)
		if err != nil {
			return nil, err
		}
		resourceInfos = append(resourceInfos, info)
	}

	return resourceInfos, nil
}

func (g *kubeClientAdapter) convertToInfo(unstruct *unstructured.Unstructured) (*resource.Info, error) {
	info := &resource.Info{}
	gvk := unstruct.GroupVersionKind()
	gv := gvk.GroupVersion()
	client, err := newRestClient(*g.restConfig, gv)
	if err != nil {
		return nil, err
	}
	info.Client = client
	restMapping, err := g.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	info.Mapping = restMapping
	info.Namespace = unstruct.GetNamespace()
	info.Name = unstruct.GetName()
	info.Object = unstruct.DeepCopyObject()
	return info, nil
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

func (g *kubeClientAdapter) deleteResource(infoTarget *resource.Info) (*Resource, error) {
	result, errs := g.helmClient.Delete(kube.ResourceList{infoTarget})
	if errs != nil || result == nil || result.Deleted == nil {
		g.logger.Warnf("kubeClient failed to delete %s '%s' (namespace: %s): %v",
			infoTarget.Object.GetObjectKind().GroupVersionKind().Kind, infoTarget.Name, infoTarget.Namespace, errs)
		return nil, errs[0]
	}
	infoDeleted := result.Deleted.Get(infoTarget)
	if infoDeleted == nil {
		g.logger.Errorf("Deleted Resource mismatch: Target: %s '%s' (namespace: %s) - Actual:  %s '%s' (namespace: %s)",
			infoTarget.Object.GetObjectKind().GroupVersionKind().Kind, infoTarget.Name, infoTarget.Namespace,
			infoDeleted.Object.GetObjectKind().GroupVersionKind().Kind, infoDeleted.Name, infoDeleted.Namespace)
		return nil, fmt.Errorf("Deleted Resource mismatch")
	}
	g.logger.Debugf("kubeClient delete %s '%s' (namespace: %s) successfully.",
		infoDeleted.Object.GetObjectKind().GroupVersionKind().Kind, infoDeleted.Name, infoDeleted.Namespace)
	return &Resource{
		Kind:      infoDeleted.Object.GetObjectKind().GroupVersionKind().Kind,
		Name:      infoDeleted.Name,
		Namespace: infoDeleted.Namespace,
	}, nil
}

func (g *kubeClientAdapter) deployResource(infoOriginal, infoTarget *resource.Info, namespaceOverride string) (*Resource, error) {

	helper := resource.NewHelper(infoTarget.Client, infoTarget.Mapping)
	setNamespaceIfScoped(namespaceOverride, infoTarget, helper)

	strategy, err := newDefaultUpdateStrategyResolver(helper).Resolve(infoTarget)
	if err != nil {
		return nil, err
	}

	if strategy == SkipUpdateStrategy {
		return nil, nil
	}

	//retry the reconciliation in case of an error
	err = retry.Do(g.deployResourceFunc(infoOriginal, infoTarget, strategy),
		retry.Attempts(uint(g.config.MaxRetries)),
		retry.Delay(g.config.RetryDelay),
		retry.LastErrorOnly(false),
		retry.Context(context.Background()))

	if err != nil {
		return nil, errors.Wrapf(err, "kubeClient failed to update %s '%s' (namespace: %s)",
			infoTarget.Object.GetObjectKind().GroupVersionKind().Kind, infoTarget.Name, infoTarget.Namespace)
	}

	resource := &Resource{
		Kind:      infoTarget.Object.GetObjectKind().GroupVersionKind().Kind,
		Name:      infoTarget.Name,
		Namespace: infoTarget.Namespace,
	}
	return resource, nil
}

func (g *kubeClientAdapter) deployResourceFunc(infoOriginal, infoTarget *resource.Info, strategy UpdateStrategy) func() error {
	return func() error {
		replaceResource := strategy == ReplaceUpdateStrategy
		_, err := g.helmClient.Update(kube.ResourceList{infoOriginal}, kube.ResourceList{infoTarget}, replaceResource)
		if err == nil {
			g.logger.Debugf("kubeClient updated %s '%s' (namespace: %s) with stategy '%s' successfully ",
				infoTarget.Object.GetObjectKind().GroupVersionKind().Kind, infoTarget.Name, infoTarget.Namespace, strategy)
		} else {
			g.logger.Warnf("kubeClient failed to update %s '%s' (namespace: %s)  with strategy '%s': %s",
				infoTarget.Object.GetObjectKind().GroupVersionKind().Kind, infoTarget.Name, infoTarget.Namespace, strategy, err)
		}
		return err
	}
}

func setDefaultNamespaceIfScopedAndNoneSet(namespace string, resourceInfo *resource.Info, helper *resource.Helper) {
	if helper.NamespaceScoped {
		resNamespace := resourceInfo.Namespace
		if resNamespace == "" {
			if namespace == "" {
				namespace = "default"
			}
			resourceInfo.Namespace = namespace
		}
	}
}

func setNamespaceIfScoped(namespace string, resourceInfo *resource.Info, helper *resource.Helper) {
	setDefaultNamespaceIfScopedAndNoneSet(namespace, resourceInfo, helper)
	if resourceInfo.Namespace == "" && helper.NamespaceScoped {
		resourceInfo.Namespace = namespace
	}
}

func (g *kubeClientAdapter) addNamespaceUnstruct(unstructs []*unstructured.Unstructured, namespace string) ([]*unstructured.Unstructured, error) {
	if namespace == defaultNamespace {
		//default namespace always exists: nothing to do
		return unstructs, nil
	}

	//check if the namespace resource is already defined in the manifest
	for _, unstruct := range unstructs {
		if strings.ToLower(unstruct.GetKind()) == "namespace" && unstruct.GetName() == namespace {
			g.logger.Debugf("Namespace '%s' is defined as resource in the manifest", namespace)
			return unstructs, nil
		}
	}

	//add namespace resource to manifest
	g.logger.Debugf("Namespace '%s' is missing: will add namespace resource to the beginning of the manifest", namespace)
	nsUnstruct, err := g.newNamespaceUnstruct(namespace)
	if err != nil {
		return nil, err
	}
	result := []*unstructured.Unstructured{nsUnstruct}
	result = append(result, unstructs...)
	return result, nil
}

func (g *kubeClientAdapter) newNamespaceUnstruct(namespace string) (*unstructured.Unstructured, error) {
	//create unstructured object for missing namespace
	nsUnstructs, err := ToUnstructured([]byte(namespaceManifest), true)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create unstructured object for namespace '%s'",
			namespace))
	}
	if len(nsUnstructs) != 1 {
		return nil, fmt.Errorf("illegal state: one unstructured object for namespace '%s' expected (got %d)",
			namespace, len(nsUnstructs))
	}
	nsUnstructs[0].SetName(namespace)
	return nsUnstructs[0], nil
}

func (g *kubeClientAdapter) DeleteResource(kind, name, namespace string) (*Resource, error) {
	if !g.resourceExists(kind, name, namespace) {
		return nil, nil
	}
	deletedResource, err := g.deleteResourceByKindAndNameAndNamespace(kind, name, namespace, metav1.DeleteOptions{})
	if err != nil && !k8serr.IsNotFound(err) {
		g.logger.Errorf("Failed to delete Kubernetes unstructured resource kind='%s', name='%s', namespace='%s': %s",
			kind, name, namespace, err)
		return deletedResource, err
	}
	return deletedResource, nil
}

func (g *kubeClientAdapter) deleteResourceByKindAndNameAndNamespace(kind, name, namespace string, do metav1.DeleteOptions) (*Resource, error) {
	gvk, err := g.mapper.KindFor(schema.GroupVersionResource{
		Resource: kind,
	})
	if err != nil {
		return nil, err
	}

	var isNamespaceResource = strings.ToLower(gvk.Kind) == "namespace"

	if !isNamespaceResource && namespace == "" { //set qualified namespace (except resource is of kind 'namespace')
		namespace = "default"
	}

	restMapping, err := g.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	restClient, err := newRestClient(*g.restConfig, gvk.GroupVersion())
	if err != nil {
		return nil, err
	}

	helper := resource.NewHelper(restClient, restMapping)

	if helper.NamespaceScoped {
		err = g.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Delete(context.TODO(), name, do)
	} else {
		err = g.dynamicClient.
			Resource(restMapping.Resource).
			Delete(context.TODO(), name, do)
	}

	//return deleted resource
	if isNamespaceResource {
		namespace = "" //namespace resources have always an empty namespace field
	}
	return &Resource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}, err
}

func (g *kubeClientAdapter) Delete(ctx context.Context, manifestTarget, namespace string) ([]*Resource, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}
	unstructsTarget, err := g.manifestToUnstructured(manifestTarget)
	if err != nil {
		g.logger.Errorf("Failed to process manifest data for delete: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}
	resourceInfoTarget, err := g.convertToInfoList(unstructsTarget)
	if err != nil {
		g.logger.Errorf("Failed to convert target unstructs data: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}
	pt, err := g.newProgressTracker()
	if err != nil {
		return nil, err
	}

	var deletedResources []*Resource
	for _, info := range resourceInfoTarget {
		helper := resource.NewHelper(info.Client, info.Mapping)
		setNamespaceIfScoped(namespace, info, helper)
		deletedResource, err := g.deleteResource(info)
		if err != nil {
			g.logger.Errorf("Failed to apply Kubernetes unstructured entity: %s", err)
			return nil, err
		}

		deletedResources = append(deletedResources, deletedResource)

		watchable, err := progress.NewWatchableResource(deletedResource.Kind)
		if err == nil {
			pt.AddResource(watchable, deletedResource.Namespace, deletedResource.Name)
		}
	}

	//wait until all resources were deleted
	if err := pt.Watch(ctx, progress.TerminatedState); err != nil {
		g.logger.Warnf("Watching progress of deleted resources failed: %s", err)
	}

	if err = g.DeleteNamespace(namespace); err != nil && !k8serr.IsNotFound(err) {
		g.logger.Errorf("Failed to delete namespace name='%s': %s",
			namespace, err)
		return deletedResources, err
	}
	return deletedResources, nil
}

func (g *kubeClientAdapter) DeleteNamespace(namespace string) error {
	r := cmdutil.NewFactory(NewRESTClientGetter(g.restConfig)).NewBuilder().
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
		err = g.dynamicClient.
			Resource(namespaceRes).
			Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	}
	return err
}

// check if resource exists in the cluster
func (g *kubeClientAdapter) resourceExists(kind, name, namespace string) bool {
	_, err := g.Get(kind, name, namespace)
	if k8serr.IsNotFound(err) {
		return false
	}
	return err == nil
}

// Get a manifest by resource/kind (example: 'pods' or 'pod'),
// name (example: 'my-pod'), and namespace (example: 'my-namespace').
func (g *kubeClientAdapter) Get(kind, name, namespace string) (*unstructured.Unstructured, error) {
	gvk, err := g.mapper.KindFor(schema.GroupVersionResource{Resource: kind})
	if err != nil {
		return nil, err
	}

	restMapping, err := g.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	restClient, err := newRestClient(*g.restConfig, gvk.GroupVersion())
	if err != nil {
		return nil, err
	}

	var u *unstructured.Unstructured

	helper := resource.NewHelper(restClient, restMapping)
	if helper.NamespaceScoped {
		u, err = g.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		u, err = g.dynamicClient.
			Resource(restMapping.Resource).
			Get(context.TODO(), name, metav1.GetOptions{})
	}

	return u, err
}

func (g *kubeClientAdapter) newProgressTracker() (*progress.Tracker, error) {
	clientSet, err := g.Clientset()
	if err != nil {
		return nil, err
	}
	return progress.NewProgressTracker(clientSet, g.logger, progress.ProgressConfig{
		Interval: g.config.ProgressInterval,
		Timeout:  g.config.ProgressTimeout,
	})
}

func (g *kubeClientAdapter) Clientset() (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(g.restConfig)
}

func (g *kubeClientAdapter) ListResource(resource string, lo metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	gvr, err := g.mapper.ResourceFor(schema.GroupVersionResource{Resource: resource})
	if err != nil {
		return nil, err
	}
	return g.dynamicClient.Resource(gvr).List(context.TODO(), lo)
}

func (g *kubeClientAdapter) GetDeployment(ctx context.Context, name, namespace string) (*v1apps.Deployment, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	clientset, err := g.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving deployments")
	}

	deployment, err := clientset.AppsV1().
		Deployments(namespace).
		Get(ctx, name, metav1.GetOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, nil
	}

	return deployment, err
}

func (g *kubeClientAdapter) GetStatefulSet(ctx context.Context, name, namespace string) (*v1apps.StatefulSet, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	clientset, err := g.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving statefulSet")
	}

	statefulSet, err := clientset.AppsV1().
		StatefulSets(namespace).
		Get(ctx, name, metav1.GetOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, nil
	}

	return statefulSet, err
}

func (g *kubeClientAdapter) GetSecret(ctx context.Context, name, namespace string) (*v1.Secret, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	clientset, err := g.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving secret")
	}

	secret, err := clientset.CoreV1().
		Secrets(namespace).
		Get(ctx, name, metav1.GetOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, nil
	}

	return secret, err
}

func (g *kubeClientAdapter) GetService(ctx context.Context, name, namespace string) (*v1.Service, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	clientset, err := g.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving service")
	}

	service, err := clientset.CoreV1().
		Services(namespace).
		Get(ctx, name, metav1.GetOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, nil
	}

	return service, err
}

func (g *kubeClientAdapter) GetPod(ctx context.Context, name, namespace string) (*v1.Pod, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	clientset, err := g.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving pod")
	}

	pod, err := clientset.CoreV1().
		Pods(namespace).
		Get(ctx, name, metav1.GetOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, nil
	}

	return pod, err
}

func (g *kubeClientAdapter) GetPersistentVolumeClaim(ctx context.Context, name, namespace string) (*v1.PersistentVolumeClaim, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	clientset, err := g.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving pvc")
	}

	pvc, err := clientset.CoreV1().
		PersistentVolumeClaims(namespace).
		Get(ctx, name, metav1.GetOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, nil
	}

	return pvc, err
}

func (g *kubeClientAdapter) GetJob(ctx context.Context, name, namespace string) (*batchv1.Job, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	clientset, err := g.Clientset()
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving pvc")
	}

	job, err := clientset.BatchV1().
		Jobs(namespace).
		Get(ctx, name, metav1.GetOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, nil
	}

	return job, err
}

func ResolveNamespace(resource *unstructured.Unstructured, namespace string) string {
	if resource.GetNamespace() != "" { //namespace defined in resource has precedence
		return resource.GetNamespace()
	}
	return namespace
}

func (g *kubeClientAdapter) GetHost() string {
	if g.restConfig == nil {
		return ""
	}

	return g.restConfig.Host
}
