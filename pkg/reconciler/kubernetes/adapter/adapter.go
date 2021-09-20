package adapter

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"time"

	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type kubeClientAdapter struct {
	kubeconfig string
	kubeClient kubeclient.KubeClient
	logger     *zap.SugaredLogger
	config     *Config
}

type Config struct {
	ProgressInterval time.Duration
	ProgressTimeout  time.Duration
}

func NewKubernetesClient(kubeconfig string, logger *zap.SugaredLogger, config *Config) (k8s.Client, error) {
	//get kubeClient
	client, err := kubeclient.NewKubeClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = &Config{}
	}

	return &kubeClientAdapter{
		kubeconfig: kubeconfig,
		kubeClient: *client,
		logger:     logger,
		config:     config,
	}, nil
}

func (g *kubeClientAdapter) Kubeconfig() string {
	return g.kubeconfig
}

func (g *kubeClientAdapter) PatchUsingStrategy(kind, name, namespace string, p []byte, strategy types.PatchType) error {
	_, _, err := g.kubeClient.PatchUsingStrategy(kind, name, namespace, p, strategy)
	return err
}

func (g *kubeClientAdapter) Deploy(ctx context.Context, manifest, namespace string, interceptors ...k8s.ResourceInterceptor) ([]*k8s.Resource, error) {
	if namespace == "" {
		namespace = "default"
	}

	//ensure namespace exists
	clientset, err := g.Clientset()
	if err != nil {
		return nil, err
	}
	_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		g.logger.Debugf("Namespace '%s' is required to deploy manifest and already exists", namespace)
	} else {
		if k8serr.IsNotFound(err) {
			if err := g.createNamespace(ctx, clientset, namespace); err != nil {
				g.logger.Errorf("Failed to create namespace '%s' which is required to deploy manifest: %s",
					namespace, err)
				return nil, err
			}
			g.logger.Debugf("Namespace '%s' is required to deploy manifest and was successfully created", namespace)
		} else {
			return nil, errors.Wrap(err,
				fmt.Sprintf("Failed to get namespace '%s' which is required to deploy manifest", namespace))
		}
	}

	deployedResources, err := g.deployManifest(ctx, manifest, namespace, interceptors)

	//delete namespace if no resources was deployed into it
	if len(deployedResources) == 0 {
		g.logger.Warnf("Namespace '%s' was required for deploying the manifest "+
			"but no resources were finally deployed into it", namespace)
	}

	return deployedResources, err
}

func (g *kubeClientAdapter) createNamespace(ctx context.Context, client kubernetes.Interface, namespace string) error {
	_, err := client.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	return err
}

func (g *kubeClientAdapter) deployManifest(ctx context.Context, manifest, namespace string, interceptors []k8s.ResourceInterceptor) ([]*k8s.Resource, error) {
	var deployedResources []*k8s.Resource

	pt, err := g.newProgressTracker()
	if err != nil {
		return nil, err
	}

	unstructs, err := kubeclient.ToUnstructured([]byte(manifest), true)
	if err != nil {
		g.logger.Errorf("Failed to process manifest file: %s", err)
		g.logger.Debugf("Manifest file: %s", manifest)
		return nil, err
	}

	for _, unstruct := range unstructs {
		for _, interceptor := range interceptors {
			if interceptor == nil {
				continue
			}

			if err := interceptor.Intercept(unstruct); err != nil {
				g.logger.Errorf("Failed to intercept Kubernetes unstructured entity: %s", err)
				return deployedResources, err
			}
		}
		resource, err := g.kubeClient.ApplyWithNamespaceOverride(unstruct, namespace)
		if err != nil {
			g.logger.Errorf("Failed to apply Kubernetes unstructured entity: %s", err)
			g.logger.Debugf("Used JSON data: %+v", unstruct)
			return deployedResources, err
		}

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

func (g *kubeClientAdapter) Delete(ctx context.Context, manifest, namespace string) ([]*k8s.Resource, error) {
	if namespace == "" {
		namespace = "default"
	}

	unstructs, err := kubeclient.ToUnstructured([]byte(manifest), true)
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
	var deletedResources []*k8s.Resource
	for i := len(unstructs) - 1; i >= 0; i-- {
		unstruct := unstructs[i]

		g.logger.Debugf("Deleting resource kind='%s', name='%s', namespace='%s'",
			unstruct.GetKind(), unstruct.GetName(), unstruct.GetNamespace())

		resource, err := g.kubeClient.DeleteResourceByKindAndNameAndNamespace(
			unstruct.GetKind(), unstruct.GetName(), namespace, metav1.DeleteOptions{})
		if err != nil && !k8serr.IsNotFound(err) {
			g.logger.Errorf("Failed to delete Kubernetes unstructured resource kind='%s', name='%s', namespace='%s': %s",
				unstruct.GetKind(), unstruct.GetName(), unstruct.GetNamespace(), err)
			return deletedResources, err
		}

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

	if err = g.kubeClient.DeleteNamespace(namespace); err != nil {
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
