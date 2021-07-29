package kubernetes

import (
	"bufio"
	"bytes"
	b64 "encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	yamlToJson "sigs.k8s.io/yaml"
)

const (
	envVarKubectlPath = "KUBECTL_PATH"
)

type kubeClientAdapter struct {
	kubeClient     KubeClient
	logger         *zap.SugaredLogger
	kubectlCmd     string
	kubeconfigFile string
	manifestFile   string
}

func newKubeClientAdapter(kubeconfig string, debug bool) (Client, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}

	//get kubectlcmd
	kubectlCmd, err := kubectl()
	if err != nil {
		return nil, err
	}

	//persist kubeconfig for kubectl-calls
	kubeconfigFile := "/tmp/kubeconfig-" + uuid.New().String()
	if err := ioutil.WriteFile(kubeconfigFile, []byte(kubeconfig), 0600); err != nil {
		return nil, err
	}

	//get kubeClient
	base64kubeConfig := b64.StdEncoding.EncodeToString([]byte(kubeconfig))
	client, err := NewKubeClient(base64kubeConfig)
	if err != nil {
		return nil, err
	}

	return &kubeClientAdapter{
		kubeClient:     *client,
		logger:         logger,
		kubectlCmd:     kubectlCmd,
		kubeconfigFile: kubeconfigFile,
	}, nil
}

func (g *kubeClientAdapter) Deploy(manifest string, interceptors ...ResourceInterceptor) ([]*Resource, error) {
	var deployedResources []*Resource

	chanMes, chanErr := g.readYaml([]byte(manifest))
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

func (g *kubeClientAdapter) Clientset() (*kubernetes.Clientset, error) {
	return g.kubeClient.GetClientSet()
}

func (g *kubeClientAdapter) readYaml(data []byte) (<-chan []byte, <-chan error) {
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

// TODO change implementation to native go
func (g *kubeClientAdapter) Delete(manifest string) error {
	//store manifest as file
	manifestFile, err := g.getManifestFile(manifest)
	if err != nil {
		return err
	}
	//call kubectl delete
	args := []string{fmt.Sprintf("--kubeconfig=%s", g.kubeconfigFile), "delete", "-f", manifestFile}
	//nolint:gosec //arguments for cmd call not allowed: replace command-call with Go k8s-client
	_, err = exec.Command(g.kubectlCmd, args...).CombinedOutput()
	return err
}

func (g *kubeClientAdapter) getManifestFile(manifest string) (string, error) {
	if g.manifestFile == "" {
		g.manifestFile = "/tmp/manifest-" + uuid.New().String()
		if err := ioutil.WriteFile(g.manifestFile, []byte(manifest), 0600); err != nil {
			return "", errors.Wrap(err, "failed to store manifests in a file")
		}
	}
	return g.manifestFile, nil
}

func kubectl() (string, error) {
	//try lookup using which
	whichLookup, err := exec.Command("which", "kubectl").CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(whichLookup)), nil
	}
	//fallback to env-var
	envLookup, ok := os.LookupEnv(envVarKubectlPath)
	if !ok {
		return "", fmt.Errorf("cannot find kubectl cmd, please set env-var '%s'", envVarKubectlPath)
	}
	return envLookup, nil
}
