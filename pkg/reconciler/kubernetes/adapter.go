package kubernetes

import (
	"context"
	"fmt"
	"time"

	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/internal"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
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
)

type kubeClientAdapter struct {
	kubeconfig string
	kubeClient *internal.KubeClient
	logger     *zap.SugaredLogger
	config     *Config
}

type Config struct {
	ProgressInterval time.Duration
	ProgressTimeout  time.Duration
}

func NewKubernetesClient(kubeconfig string, logger *zap.SugaredLogger, config *Config) (Client, error) {
	kubeClient, err := internal.NewKubeClient(kubeconfig, logger)
	if err != nil {
		return nil, err
	}

	return adapt(kubeClient, kubeconfig, logger, config), err
}

func NewInClusterClientSet(logger *zap.SugaredLogger) (kubernetes.Interface, error) {
	inClusterClient, err := internal.NewInClusterClient(logger)
	if err != nil {
		logger.Infof("Not able to create an In Cluster Client")
		return nil, nil
	}

	inClusterClientSet, err := inClusterClient.GetClientSet()
	if err != nil {
		return nil, err
	}

	return inClusterClientSet, nil
}

func adapt(impl *internal.KubeClient, kubeconfig string, logger *zap.SugaredLogger, config *Config) *kubeClientAdapter {
	if config == nil {
		config = &Config{}
	}

	return &kubeClientAdapter{
		kubeconfig: kubeconfig,
		kubeClient: impl,
		logger:     logger,
		config:     config,
	}
}

func (g *kubeClientAdapter) Kubeconfig() string {
	return g.kubeconfig
}

func (g *kubeClientAdapter) PatchUsingStrategy(kind, name, namespace string, p []byte, strategy types.PatchType) error {
	_, _, err := g.kubeClient.PatchUsingStrategy(kind, name, namespace, p, strategy)
	return err
}

func (g *kubeClientAdapter) Deploy(ctx context.Context, manifest, namespace string, interceptors ...ResourceInterceptor) ([]*Resource, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	deployedResources, err := g.deployManifest(ctx, manifest, namespace, interceptors)

	//delete namespace if no resources was deployed into it
	if len(deployedResources) == 0 {
		g.logger.Warnf("Namespace '%s' was required for deploying the manifest "+
			"but no resources were finally deployed into it", namespace)
	}

	return deployedResources, err
}

func (g *kubeClientAdapter) deployManifest(ctx context.Context, manifest, namespace string, interceptors []ResourceInterceptor) ([]*Resource, error) {
	var deployedResources []*Resource

	pt, err := g.newProgressTracker()
	if err != nil {
		return nil, err
	}

	unstructs, err := ToUnstructured([]byte(manifest), true)
	if err != nil {
		g.logger.Errorf("Failed to process manifest data: %s", err)
		g.logger.Debugf("Manifest data: %s", manifest)
		return nil, err
	}

	unstructs, err = g.addNamespaceUnstruct(unstructs, namespace)
	if err != nil {
		return nil, err
	}

LoopUnstructs:
	for _, unstruct := range unstructs {
		for _, interceptor := range interceptors {
			if interceptor == nil {
				continue
			}

			result, err := interceptor.Intercept(unstruct, namespace)
			if err != nil {
				g.logger.Warnf("One of the interceptors returned interception result '%s' with an error while "+
					"processing Kubernetes unstructured entity '%s@%s' (kind '%s'): %s",
					result, unstruct.GetName(), unstruct.GetNamespace(), unstruct.GetKind(), err)
			}
			switch result {
			case ErrorInterceptionResult:
				return deployedResources, err
			case IgnoreResourceInterceptionResult:
				g.logger.Debugf("Interceptor indicated to not apply Kuberentes resource '%s@%s' (kind '%s')",
					unstruct.GetName(), unstruct.GetNamespace(), unstruct.GetKind())
				continue LoopUnstructs //do not apply this resource and continue with next one
			default:
				//continue change: just do nothing and continue processing
			}
		}
		metadata, err := g.kubeClient.ApplyWithNamespaceOverride(unstruct, namespace)
		if err != nil {
			g.logger.Errorf("Failed to apply Kubernetes unstructured entity: %s", err)
			g.logger.Debugf("Used JSON data: %+v", unstruct)
			return deployedResources, err
		}

		resource := toResource(metadata)

		//add deploy resource to result
		g.logger.Debugf("Kubernetes resource '%v' successfully deployed", resource)
		deployedResources = append(deployedResources, resource)

		//if resource is watchable, add it to progress tracker
		watchable, err := progress.NewWatchableResource(resource.Kind)
		if err == nil { //add only watchable resources to progress tracker
			pt.AddResource(watchable, resource.Namespace, resource.Name)
		}
	}

	g.logger.Debugf("Manifest processed: %d Kubernetes resources were successfully deployed",
		len(deployedResources))
	return deployedResources, pt.Watch(ctx, progress.ReadyState)
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

func (g *kubeClientAdapter) Delete(ctx context.Context, manifest, namespace string) ([]*Resource, error) {
	if namespace == "" {
		namespace = defaultNamespace
	}

	unstructs, err := ToUnstructured([]byte(manifest), true)
	if err != nil {
		g.logger.Errorf("Failed to process manifest file: %s", err)
		g.logger.Debugf("Manifest file: %s", manifest)
		return nil, err
	}

	pt, err := g.newProgressTracker()
	if err != nil {
		return nil, err
	}

	//delete resource in reverse order
	var deletedResources []*Resource
	for i := len(unstructs) - 1; i >= 0; i-- {
		unstruct := unstructs[i]

		g.logger.Debugf("Deleting resource kind='%s', name='%s', namespace='%s'",
			unstruct.GetKind(), unstruct.GetName(), unstruct.GetNamespace())

		metadata, err := g.kubeClient.DeleteResourceByKindAndNameAndNamespace(
			unstruct.GetKind(), unstruct.GetName(), namespace, metav1.DeleteOptions{})
		if err != nil && !k8serr.IsNotFound(err) {
			g.logger.Errorf("Failed to delete Kubernetes unstructured resource kind='%s', name='%s', namespace='%s': %s",
				unstruct.GetKind(), unstruct.GetName(), unstruct.GetNamespace(), err)
			return deletedResources, err
		}

		resource := toResource(metadata)

		//add deleted resource to result set
		deletedResources = append(deletedResources, resource)

		//if resource is watchable, add it to progress tracker
		watchable, err := progress.NewWatchableResource(resource.Kind)
		if err == nil { //add only watchable resources to progress tracker
			pt.AddResource(watchable, resource.Namespace, resource.Name)
		}
	}

	//wait until all resources were deleted
	if err := pt.Watch(ctx, progress.TerminatedState); err != nil {
		g.logger.Warnf("Watching progress of deleted resources failed: %s", err)
	}

	if err = g.kubeClient.DeleteNamespace(namespace); err != nil && !k8serr.IsNotFound(err) {
		g.logger.Errorf("Failed to delete namespace name='%s': %s",
			namespace, err)
		return deletedResources, err
	}
	return deletedResources, nil
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
	return g.kubeClient.GetClientSet()
}

func (g *kubeClientAdapter) ListResource(resource string, lo metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return g.kubeClient.ListResource(resource, lo)
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

func toResource(m *internal.Metadata) *Resource {
	return &Resource{
		Name:      m.Name,
		Kind:      m.Kind,
		Namespace: m.Namespace,
	}
}

func ToUnstructured(manifest []byte, async bool) ([]*unstructured.Unstructured, error) {
	// expose the internal unstructured converter
	return internal.ToUnstructured(manifest, async)
}
