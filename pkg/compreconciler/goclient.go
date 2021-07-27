package compreconciler

import (
	"bufio"
	"bytes"
	b64 "encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/compreconciler/kubeClient"
	"github.com/kyma-incubator/reconciler/pkg/compreconciler/types"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"os"
	"os/exec"
	yamlToJson "sigs.k8s.io/yaml"
	"strings"
)

type goClient struct {
	kubeClient kubeClient.KubeClient

	kubectlCmd     string
	kubeconfigFile string
	manifestFile   string
}

const (
	envVarKubectlPath = "KUBECTL_PATH"
)

func newGoClient(kubeconfig string) (kubernetesClient, error) {
	kubectlCmd, err := kubectl()
	if err != nil {
		return nil, err
	}
	kubeconfigFile := "/tmp/kubeconfig-" + uuid.New().String()
	if err := ioutil.WriteFile(kubeconfigFile, []byte(kubeconfig), 0600); err != nil {
		return nil, err
	}

	base64kubeConfig := b64.StdEncoding.EncodeToString([]byte(kubeconfig))
	client, err := kubeClient.NewKubeClient(base64kubeConfig)
	if err != nil {
		return nil, err
	}

	return &goClient{
		kubeClient: *client,

		kubectlCmd:     kubectlCmd,
		kubeconfigFile: kubeconfigFile,
	}, nil
}

func (g goClient) Deploy(manifest string) (results []string, resources []types.Metadata, err error) {
	chanMes, chanErr := readYaml([]byte(manifest))
	for {
		select {
		case dataBytes, ok := <-chanMes:
			{
				if !ok {
					return results, resources, err
				}
				if err != nil {
					results = append(results, err.Error())
					continue
				}

				json, err := yamlToJson.YAMLToJSON(dataBytes)
				toUnstructured, err := kubeClient.ToUnstructured(json)
				if err == nil {
					resource, err := g.kubeClient.Apply(&toUnstructured)
					if err == nil {
						resources = append(resources, resource)
						continue
					}
					results = append(results, err.Error())
				}

				// Get obj and dr
				//obj, dr, err := buildDynamicResourceClient(kubeClient, dataBytes)
				//if err != nil {
				//	result = append(result, err.Error())
				//	continue
				//}
				//
				//// Create or Update
				//_, err = dr.Patch(ctx, obj.GetName(), types.ApplyPatchType, dataBytes, metav1.PatchOptions{
				//	FieldManager: "kubectl-golang",
				//})
				//if err != nil {
				//	result = append(result, err.Error())
				//} else {
				//	result = append(result, obj.GetName()+" patched.")
				//}
			}
		case err, ok := <-chanErr:
			if !ok {
				return results, resources, err
			}
			if err == nil {
				continue
			}
			results = append(results, err.Error())
		}
	}
}

func (g goClient) Clientset() (*kubernetes.Clientset, error) {
	return g.kubeClient.GetClientSet()
}

func readYaml(data []byte) (<-chan []byte, <-chan error) {
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
func (g goClient) Delete(manifest string) error {
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

func (g *goClient) getManifestFile(manifest string) (string, error) {
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
