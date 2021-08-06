package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	b64 "encoding/base64"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/progress"
	"go.uber.org/zap"
	"io"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"time"

	"github.com/pkg/errors"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	yamlToJson "sigs.k8s.io/yaml"
)

type kubeClientAdapter struct {
	kubeClient KubeClient
	logger     *zap.SugaredLogger
}

func newKubeClientAdapter(kubeconfig string, logger *zap.SugaredLogger) (Client, error) {
	//get kubeClient
	base64kubeConfig := b64.StdEncoding.EncodeToString([]byte(kubeconfig))
	client, err := NewKubeClient(base64kubeConfig)
	if err != nil {
		return nil, err
	}

	return &kubeClientAdapter{
		kubeClient: *client,
		logger:     logger,
	}, nil
}

func (g *kubeClientAdapter) Deploy(manifest string, interceptors ...ResourceInterceptor) ([]*Resource, error) {
	var deployedResources []*Resource

	chanMes, chanErr := asyncReadYaml([]byte(manifest))
	for {
		select {
		case yamlData, ok := <-chanMes:
			if !ok {
				//channel closed
				g.logger.Debugf("Manifest processed: %d Kubernetes resources were successfully deployed",
					len(deployedResources))
				return deployedResources, nil
			}

			//convert YAML to JSON
			jsonData, err := yamlToJson.YAMLToJSON(yamlData)
			if err != nil {
				g.logger.Errorf("Failed to convert manifest YAML to JSON: %s", err)
				g.logger.Debugf("Used YAML data: %s", string(yamlData))
				return deployedResources, err
			}
			if string(jsonData) == "null" {
				//YAML didn't contain any valuable JSON data (e.g. just comments)
				g.logger.Debugf("Ignoring non-valuable manifest data '%s'", string(jsonData))
				continue
			}

			//get unstructured entity from JSON and intercept
			unstruct, err := ToUnstructured(jsonData)
			if err != nil {
				g.logger.Errorf("Failed to convert JSON to Kubernetes unstructured entity: %s", err)
				g.logger.Debugf("Used JSON data: %s", string(jsonData))
				return deployedResources, err
			}

			//intercept unstructured entity before deploying it
			for _, interceptor := range interceptors {
				if err := interceptor.Intercept(&unstruct); err != nil {
					g.logger.Errorf("Failed to intercept Kubernetes unstructured entity: %s", err)
					return deployedResources, err
				}
			}

			//deploy unstructured entity
			resource, err := g.kubeClient.Apply(&unstruct)
			if err != nil {
				g.logger.Errorf("Failed to apply Kubernetes unstructured entity: %s", err)
				return deployedResources, err
			}

			//add deploy resource to result
			g.logger.Debugf("Kubernetes resource '%v' successfully deployed", resource)
			deployedResources = append(deployedResources, resource)
		case err := <-chanErr:
			//stop processing in any error case
			return deployedResources, err
		}
	}
}

func (g kubeClientAdapter) Delete(manifest string) ([]*Resource, error) {
	yamls, err := syncReadYaml([]byte(manifest))
	if err != nil {
		g.logger.Error("Problem with read manifest")
		g.logger.Debugf("Manifest file: %s", manifest)
		return nil, err
	}

	//delete resource in reverse order
	clientSet, err := g.Clientset()
	if err != nil {
		return nil, err
	}
	pt, err := progress.NewProgressTracker(clientSet, g.logger, progress.Config{
		Interval: 2 * time.Second,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	var deletedResources []*Resource
	for i := len(yamls) - 1; i >= 0; i-- {
		json, err := yamlToJson.YAMLToJSON(yamls[i])
		if string(json) == "null" {
			g.logger.Debugf("Ignoring YAML at posistion %d which does not include payload data (only comments)", i)
			continue
		}
		if err != nil {
			g.logger.Errorf("Failed to convert manifest YAML to JSON: %s", err)
			g.logger.Debugf("Used YAML data: %s", string(json))
			return nil, err
		}
		toUnstructured, err := ToUnstructured(json)
		if err != nil {
			g.logger.Errorf("Failed to convert JSON to Kubernetes unstructured entity: %s", err)
			g.logger.Debugf("Used JSON data: %s", string(json))
			return nil, err
		}

		g.logger.Debugf("Deleting resource kind='%s', name='%s', namespace='%s'",
			toUnstructured.GetKind(), toUnstructured.GetName(), toUnstructured.GetNamespace())

		resource, err := g.kubeClient.DeleteResourceByKindAndNameAndNamespace(
			toUnstructured.GetKind(), toUnstructured.GetName(), toUnstructured.GetNamespace(), v1.DeleteOptions{})

		if err != nil && !k8serr.IsNotFound(err) {
			g.logger.Errorf("Failed to delete Kubernetes unstructured entity kind='%s', name='%s', namespace='%s': %s",
				toUnstructured.GetKind(), toUnstructured.GetName(), toUnstructured.GetNamespace(), err)
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
	if err := pt.Watch(context.TODO(), progress.TerminatedState); err != nil {
		g.logger.Warnf("Watching progress of deleted resources failed: %s", err)
	}

	return deletedResources, nil
}

func (g *kubeClientAdapter) Clientset() (kubernetes.Interface, error) {
	return g.kubeClient.GetClientSet()
}

func (g *kubeClientAdapter) Config() *rest.Config {
	return g.kubeClient.Config
}

func asyncReadYaml(data []byte) (<-chan []byte, <-chan error) {
	var (
		chanErr        = make(chan error)
		chanBytes      = make(chan []byte)
		multidocReader = utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	)

	go func() {
		defer close(chanErr)
		defer close(chanBytes)

		for {
			buf, err := multidocReader.Read()
			if err != nil {
				if err == io.EOF {
					return
				}
				chanErr <- errors.Wrap(err, "failed to read yaml data")
				return
			}
			chanBytes <- buf
		}
	}()
	return chanBytes, chanErr
}

func syncReadYaml(data []byte) (results [][]byte, err error) {
	var (
		multidocReader = utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	)
	for {
		buf, err := multidocReader.Read()
		if err != nil {
			if err == io.EOF {
				return results, nil
			}
			return results, err
		}
		results = append(results, buf)
	}
}
