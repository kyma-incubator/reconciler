package kubernetes

import (
	"bufio"
	"bytes"
	b64 "encoding/base64"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
	"io"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pkg/errors"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	yamlToJson "sigs.k8s.io/yaml"
)

type kubeClientAdapter struct {
	kubeClient KubeClient
	logger     *zap.SugaredLogger
}

func newKubeClientAdapter(kubeconfig string, debug bool) (Client, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}

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
				g.logger.Debug("Manifest processed: all Kubernetes resources were successfully deployed")
				return deployedResources, nil
			}

			//convert YAML to JSON
			jsonData, err := yamlToJson.YAMLToJSON(yamlData)
			if err != nil {
				g.logger.Error("Failed to convert manifest YAML to JSON: %s", err)
				g.logger.Debug("Used YAML data: %s", string(yamlData))
				return deployedResources, err
			}
			if string(jsonData) == "null" {
				//YAML didn't contain any valuable JSON data (e.g. just comments)
				g.logger.Debug("Ignoring non-valuable manifest data '%s'", string(jsonData))
				continue
			}

			//get unstructured entity from JSON and intercept
			unstruct, err := ToUnstructured(jsonData)
			if err != nil {
				g.logger.Error("Failed to convert JSON to Kubernetes unstructured entity: %s", err)
				g.logger.Debug("Used JSON data: %s", string(jsonData))
				return deployedResources, err
			}

			//intercept unstructured entity before deploying it
			for _, interceptor := range interceptors {
				if err := interceptor.Intercept(&unstruct); err != nil {
					g.logger.Error("Failed to intercept Kubernetes unstructured entity: %s", err)
					return deployedResources, err
				}
			}

			//deploy unstructured entity
			resource, err := g.kubeClient.Apply(&unstruct)
			if err != nil {
				g.logger.Error("Failed to apply Kubernetes unstructured entity: %s", err)
				return deployedResources, err
			}

			//add deploy resource to result
			g.logger.Debug("Kubernetes resource '%v' successfully deployed", resource)
			deployedResources = append(deployedResources, resource)
		case err := <-chanErr:
			//stop processing in any error case
			return deployedResources, err
		}
	}
}

func (g kubeClientAdapter) Delete(manifest string) (err error) {
	yamls, err := syncReadYaml([]byte(manifest))
	if err != nil {
		g.logger.Error("Problem with read manifest")
		g.logger.Debug("Manifest file: %s", manifest)
		return err
	}

	//delete resource in reverse order
	for i := len(yamls) - 1; i >= 0; i-- {
		json, err := yamlToJson.YAMLToJSON(yamls[i])
		if err != nil {
			g.logger.Error("Failed to convert manifest YAML to JSON: %s", err)
			g.logger.Debug("Used YAML data: %s", string(json))
			return err
		}
		toUnstructured, err := ToUnstructured(json)
		if err != nil {
			g.logger.Error("Failed to convert JSON to Kubernetes unstructured entity: %s", err)
			g.logger.Debug("Used JSON data: %s", string(json))
			return err
		}
		err = g.kubeClient.DeleteResourceByKindAndNameAndNamespace(toUnstructured.GetKind(), toUnstructured.GetName(), toUnstructured.GetNamespace(), v1.DeleteOptions{})
		if err != nil {
			g.logger.Error("Failed to delete Kubernetes unstructured entity: %s", err)
			return err
		}
	}
	return nil
}

func (g *kubeClientAdapter) Clientset() (*kubernetes.Clientset, error) {
	return g.kubeClient.GetClientSet()
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
