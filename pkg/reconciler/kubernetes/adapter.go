package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"helm.sh/helm/v3/pkg/kube"
	batchv1 "k8s.io/api/batch/v1"
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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apixV1ClientSet "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
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
)

type kubeClientAdapter struct {
	kubeconfig    string
	logger        *zap.SugaredLogger
	config        *Config
	restConfig    *rest.Config
	mapper        *restmapper.DeferredDiscoveryRESTMapper
	helmClient    *kube.Client
	dynamicClient dynamic.Interface
	apixClient    apixV1ClientSet.ApiextensionsV1Interface
}

func NewKubernetesClient(kubeconfig string, logger *zap.SugaredLogger, config *Config) (Client, error) {
	if config == nil {
		config = &Config{}
	}
	err := config.validate()
	if err != nil {
		return nil, err
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
	apixClient, err := apixV1ClientSet.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return adapt(kubeconfig, logger, config, restConfig, mapper, dynamicClient, apixClient), err
}

func NewInClusterClientSet(logger *zap.SugaredLogger) (kubernetes.Interface, error) {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Infof("Not able to create an In Cluster Client")
		return nil, nil
	}
	return kubernetes.NewForConfig(inClusterConfig)
}

func adapt(kubeconfig string, logger *zap.SugaredLogger, config *Config, restConfig *rest.Config, mapper *restmapper.DeferredDiscoveryRESTMapper, dynamicClient dynamic.Interface, apixClient *apixV1ClientSet.ApiextensionsV1Client) *kubeClientAdapter {
	return &kubeClientAdapter{
		kubeconfig:    kubeconfig,
		logger:        logger,
		config:        config,
		restConfig:    restConfig,
		mapper:        mapper,
		dynamicClient: dynamicClient,
		helmClient:    kube.New(NewRESTClientGetter(restConfig)),
		apixClient:    apixClient,
	}
}

func (g *kubeClientAdapter) Kubeconfig() string {
	return g.kubeconfig
}

func (g *kubeClientAdapter) PatchUsingStrategy(context context.Context, kind, name, namespace string, p []byte, strategy types.PatchType) error {
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

	if helper.NamespaceScoped {
		_, err = g.dynamicClient.
			Resource(restMapping.Resource).
			Namespace(namespace).
			Patch(context, name, strategy, p, metav1.PatchOptions{})
	} else {
		_, err = g.dynamicClient.
			Resource(restMapping.Resource).
			Patch(context, name, strategy, p, metav1.PatchOptions{})
	}

	if err != nil {
		return err
	}

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

	unstructsTarget, err := g.applyInterceptors(manifestTarget, namespace, interceptors)
	if err != nil {
		g.logger.Errorf("Failed to process target manifest data for deploy: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}
	resourceInfoTarget, err := g.filterAndConvertToInfoList(unstructsTarget, namespace, false)
	if err != nil {
		g.logger.Errorf("Failed to convert target unstructs data: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}
	deployedResources, err := g.deployResources(ctx, resourceInfoOriginal, resourceInfoTarget, nil)

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

	unstructsTarget, err := g.applyInterceptors(manifestTarget, namespace, interceptors)
	if err != nil {
		g.logger.Errorf("Failed to process target manifest data for deploy: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}
	resourceInfoTarget, err := g.filterAndConvertToInfoList(unstructsTarget, namespace, false)
	if err != nil {
		g.logger.Errorf("Failed to convert target unstructs data: %s", err)
		g.logger.Debugf("Manifest data: %s", manifestTarget)
		return nil, err
	}
	crDGroupKinds, err := g.getCRDGroupKinds(ctx)
	if err != nil {
		return nil, err
	}
	deployedResources, err := g.deployResources(ctx, resourceInfoTarget, resourceInfoTarget, crDGroupKinds)

	if len(deployedResources) == 0 {
		g.logger.Warnf("Namespace '%s' was required for deploying the manifestTarget "+
			"but no resources were finally deployed into it", namespace)
	}

	return deployedResources, err
}

func (g *kubeClientAdapter) applyInterceptors(manifestTarget string, namespace string, interceptors []ResourceInterceptor) ([]*unstructured.Unstructured, error) {

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
	return resourceListTarget.resources, nil
}

func (g *kubeClientAdapter) deployResources(ctx context.Context, infoOriginalList kube.ResourceList, infoTargetList kube.ResourceList, crdGroupKinds []schema.GroupKind) ([]*Resource, error) {
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

		deployingResource := g.addWatchableResourceInfoToProgressTracker(infoTarget, pt)
		deployedResources = append(deployedResources, deployingResource)

		err = g.deployResource(ctx, intersectOriginal[0], infoTarget, crdGroupKinds)
		if err != nil {
			g.logger.Errorf("Failed to apply Kubernetes unstructured entity: %s", err)
			return nil, err
		}
		g.logger.Debugf("Kubernetes deployingResource '%v' successfully deployed", deployingResource)
	}

	return deployedResources, pt.Watch(ctx, progress.ReadyState)
}

func (g *kubeClientAdapter) getUpdateStrategy(infoTarget *resource.Info) (UpdateStrategy, error) {
	helper := resource.NewHelper(infoTarget.Client, infoTarget.Mapping)
	strategy, err := newDefaultUpdateStrategyResolver(helper, g.logger).Resolve(infoTarget)
	return strategy, err
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

func (g *kubeClientAdapter) addWatchableResourceInfoToProgressTracker(info *resource.Info, pt *progress.Tracker) *Resource {
	res := &Resource{
		Name:      info.Name,
		Kind:      info.Object.GetObjectKind().GroupVersionKind().Kind,
		Namespace: info.Namespace,
	}
	watchable, nonWatchableErr := progress.NewWatchableResource(res.Kind)
	if nonWatchableErr == nil {
		pt.AddResourceWithInfo(watchable, res.Namespace, res.Name, info)
	}
	return res
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

func (g *kubeClientAdapter) filterAndConvertToInfoList(unstructs []*unstructured.Unstructured, namespaceOverride string, ignoreNotMatchError bool) ([]*resource.Info, error) {
	var resourceInfos []*resource.Info

	for _, unstruct := range unstructs {
		info, err := g.convertToInfo(unstruct, namespaceOverride)
		if ignoreNotMatchError && apiMeta.IsNoMatchError(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		resourceInfos = append(resourceInfos, info)
	}

	return resourceInfos, nil
}

func (g *kubeClientAdapter) convertToInfo(unstruct *unstructured.Unstructured, namespaceOverride string) (*resource.Info, error) {
	info := &resource.Info{}
	gvk := unstruct.GroupVersionKind()
	gv := gvk.GroupVersion()
	client, err := newRestClient(*g.restConfig, gv)
	if err != nil {
		return nil, err
	}
	info.Client = client
	err = retry.Do(g.constructRestMappingFunc(gvk, info),
		retry.Attempts(uint(5)),
		retry.Delay(1*time.Second),
		retry.LastErrorOnly(true))

	if err != nil {
		return nil, err
	}

	info.Namespace = unstruct.GetNamespace()
	helper := resource.NewHelper(info.Client, info.Mapping)
	err = setDefaultOrOverwriteNamespaceIfScopedAndNoneSet(namespaceOverride, info, helper, unstruct)
	if err != nil {
		return nil, err
	}
	info.Name = unstruct.GetName()
	info.Object = unstruct.DeepCopyObject()
	return info, nil
}

func (g *kubeClientAdapter) constructRestMappingFunc(gvk schema.GroupVersionKind, info *resource.Info) func() error {
	return func() error {
		restMapping, err := g.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			g.mapper.Reset()
			return err
		}
		info.Mapping = restMapping
		return nil
	}
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

func (g *kubeClientAdapter) deployResource(ctx context.Context, infoOriginal, infoTarget *resource.Info, crdGroupKinds []schema.GroupKind) error {

	strategy, err := g.getUpdateStrategy(infoTarget)
	if err != nil {
		return err
	}
	if strategy == SkipUpdateStrategy {
		return nil
	}

	infoOriginal, err = g.fetchExistingResourceAndConvertToInfo(ctx, infoOriginal, crdGroupKinds)
	if err != nil {
		return err
	}
	err = retry.Do(g.deployResourceFunc(infoOriginal, infoTarget, strategy),
		retry.Attempts(uint(g.config.MaxRetries)),
		retry.Delay(g.config.RetryDelay),
		retry.LastErrorOnly(false),
		retry.Context(context.Background()))

	if err != nil {
		return errors.Wrapf(err, "kubeClient failed to update %s '%s' (namespace: %s)",
			infoTarget.Object.GetObjectKind().GroupVersionKind().Kind, infoTarget.Name, infoTarget.Namespace)
	}
	return nil
}

// fetchExistingResourceAndConvertToInfo: skip non CR resources, get existing CR definitions from cluster, and convert as resource.Info
func (g *kubeClientAdapter) fetchExistingResourceAndConvertToInfo(ctx context.Context, info *resource.Info, crdGroupKinds []schema.GroupKind) (*resource.Info, error) {

	if !containsGroupKind(crdGroupKinds, info.Object.GetObjectKind().GroupVersionKind().GroupKind()) {
		return info, nil
	}
	existingResource, err := g.dynamicClient.Resource(info.Mapping.Resource).Namespace(info.Namespace).Get(ctx, info.Name, metav1.GetOptions{})
	if err != nil && !k8serr.IsNotFound(err) {
		return info, err
	}
	if existingResource != nil {
		removeIgnoredFields(existingResource)
		info, err = g.convertToInfo(existingResource, info.Namespace)
		if err != nil {
			return info, errors.Wrapf(err, "Failed to convert existing unstructs data")
		}
	}
	return info, nil
}

func removeIgnoredFields(existingResource *unstructured.Unstructured) {
	unstructured.RemoveNestedField(existingResource.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(existingResource.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(existingResource.Object, "metadata", "generation")
	unstructured.RemoveNestedField(existingResource.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(existingResource.Object, "metadata", "uid")
}

func containsGroupKind(groupKinds []schema.GroupKind, groupKind schema.GroupKind) bool {
	if groupKinds == nil {
		return false
	}
	for _, gk := range groupKinds {
		if gk.Group == groupKind.Group && gk.Kind == groupKind.Kind {
			return true
		}
	}
	return false
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

func setDefaultOrOverwriteNamespaceIfScopedAndNoneSet(namespaceOverride string, resourceInfo *resource.Info, helper *resource.Helper, unstruct *unstructured.Unstructured) error {
	if helper.NamespaceScoped {
		if resourceInfo.Namespace == "" {
			if namespaceOverride == "" {
				namespaceOverride = "default"
			}
			resourceInfo.Namespace = namespaceOverride
		}
		err := unstructured.SetNestedField(unstruct.Object, resourceInfo.Namespace, "metadata", "namespace")
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *kubeClientAdapter) getCRDGroupKinds(ctx context.Context) ([]schema.GroupKind, error) {
	res := []schema.GroupKind{}
	crds, err := g.apixClient.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil && !k8serr.IsNotFound(err) {
		return nil, err
	}

	if crds == nil {
		return res, nil
	}

	for _, crd := range crds.Items {
		gk := schema.GroupKind{
			Group: crd.Spec.Group,
			Kind:  crd.Spec.Names.Kind,
		}
		res = append(res, gk)
	}

	return res, nil
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

func (g *kubeClientAdapter) DeleteResource(context context.Context, kind, name, namespace string) (*Resource, error) {
	if !g.resourceExists(kind, name, namespace) {
		return nil, nil
	}
	deletedResource, err := g.deleteResourceByKindAndNameAndNamespace(context, kind, name, namespace, metav1.DeleteOptions{})
	if err != nil && !k8serr.IsNotFound(err) {
		g.logger.Errorf("Failed to delete Kubernetes unstructured resource kind='%s', name='%s', namespace='%s': %s",
			kind, name, namespace, err)
		return deletedResource, err
	}
	return deletedResource, nil
}

func (g *kubeClientAdapter) deleteResourceByKindAndNameAndNamespace(context context.Context, kind, name, namespace string, do metav1.DeleteOptions) (*Resource, error) {
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
			Delete(context, name, do)
	} else {
		err = g.dynamicClient.
			Resource(restMapping.Resource).
			Delete(context, name, do)
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
	resourceInfoTarget, err := g.filterAndConvertToInfoList(unstructsTarget, namespace, true)
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
		deletedResource, err := g.deleteResource(info)
		if err != nil {
			g.logger.Errorf("Failed to apply Kubernetes unstructured entity: %s", err)
			return nil, err
		}

		deletedResources = append(deletedResources, deletedResource)

		watchable, err := progress.NewWatchableResource(deletedResource.Kind)
		if err == nil {
			pt.AddResourceWithInfo(watchable, deletedResource.Namespace, deletedResource.Name, info)
		}
	}

	//wait until all resources were deleted
	if err := pt.Watch(ctx, progress.TerminatedState); err != nil {
		g.logger.Warnf("Watching progress of deleted resources failed: %s", err)
	}

	if err = g.DeleteNamespace(ctx, namespace); err != nil && !k8serr.IsNotFound(err) {
		g.logger.Errorf("Failed to delete namespace name='%s': %s",
			namespace, err)
		return deletedResources, err
	}
	return deletedResources, nil
}

func (g *kubeClientAdapter) DeleteNamespace(ctx context.Context, namespace string) error {
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

	if len(infos) == 0 {
		namespaceRes := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
		propagation := metav1.DeletePropagationForeground
		err = g.dynamicClient.
			Resource(namespaceRes).
			Delete(ctx, namespace, metav1.DeleteOptions{PropagationPolicy: &propagation})
		g.logger.Debugf("kubeClient delete namespace: %s (propagation policy %s) successfully.",
			namespace, string(propagation))
	} else {
		g.logger.Debugf("kubeClient delete namespace: %s (resources blocking deletion %s) skipped",
			namespace, infos)
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
	return progress.NewProgressTracker(clientSet, g.logger, progress.Config{
		Interval: g.config.ProgressInterval,
		Timeout:  g.config.ProgressTimeout,
	})
}

func (g *kubeClientAdapter) Clientset() (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(g.restConfig)
}

func (g *kubeClientAdapter) ListResource(context context.Context, resource string, lo metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	gvr, err := g.mapper.ResourceFor(schema.GroupVersionResource{Resource: resource})
	if err != nil {
		return nil, err
	}
	return g.dynamicClient.Resource(gvr).List(context, lo)
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
