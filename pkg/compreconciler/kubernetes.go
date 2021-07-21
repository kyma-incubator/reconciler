package compreconciler

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	k8s "github.com/kyma-incubator/reconciler/pkg/kubernetes"
	"github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"os"
	"os/exec"
	"strings"
)

const (
	envVarKubectlPath = "KUBECTL_PATH"
)

type resource struct {
	kind      string
	name      string
	namespace string
}

type kubernetesClient interface {
	Deploy(manifest string) error
	DeployedResources(manifest string) ([]resource, error)
	Delete(manifest string) error
	Clientset() (*kubernetes.Clientset, error)
}

func newKubernetesClient(kubeconfig string) (kubernetesClient, error) {
	kubectlCmd, err := kubectl()
	if err != nil {
		return nil, err
	}

	kubeconfigFile := "/tmp/kubeconfig-" + uuid.New().String()
	if err := ioutil.WriteFile(kubeconfigFile, []byte(kubeconfig), 0600); err != nil {
		return nil, err
	}

	return &kubectlClient{
		kubecltCmd:     kubectlCmd,
		kubeconfigFile: kubeconfigFile,
	}, nil
}

type kubectlClient struct {
	kubecltCmd     string
	kubeconfigFile string
	manifestFile   string
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

func (kc *kubectlClient) getManifestFile(manifest string) (string, error) {
	if kc.manifestFile == "" {
		kc.manifestFile = "/tmp/manifest-" + uuid.New().String()
		if err := ioutil.WriteFile(kc.manifestFile, []byte(manifest), 0600); err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("failed to store manifests in a file"))
		}
	}
	return kc.manifestFile, nil
}

func (kc *kubectlClient) Deploy(manifest string) error {
	//store manifest as file
	manifestFile, err := kc.getManifestFile(manifest)
	if err != nil {
		return err
	}
	//call kubectl apply
	args := []string{fmt.Sprintf("--kubeconfig=%s", kc.kubeconfigFile), "apply", "-f", manifestFile}
	output, err := exec.Command(fmt.Sprint(kc.kubecltCmd), args...).CombinedOutput()
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Exeuction of kubeclt command with args '%s' failed: %s", strings.Join(args, " "), output))
	}
	return err
}

func (kc *kubectlClient) DeployedResources(manifest string) ([]resource, error) {
	manifestFile, err := kc.getManifestFile(manifest)
	if err != nil {
		return nil, err
	}

	args := []string{"get", "-f", manifestFile, fmt.Sprintf("--kubeconfig=%s", kc.kubeconfigFile), "-ojson"}
	getCommandStout, err := exec.Command(fmt.Sprint(kc.kubecltCmd), args...).CombinedOutput()
	if err != nil {
		return nil, err
	}

	//marshal json result
	resources := make(map[string]interface{})
	err = json.Unmarshal(getCommandStout, &resources)
	if err != nil {
		return nil, err
	}

	//extract resources
	var result []resource
	for _, item := range resources["items"].([]interface{}) {
		res := item.(map[string]interface{})
		metadata := res["metadata"].(map[string]interface{})
		namespace, ok := metadata["namespace"]
		if !ok {
			namespace = ""
		}
		result = append(result, resource{
			kind:      res["kind"].(string),
			name:      metadata["name"].(string),
			namespace: namespace.(string),
		})
	}

	return result, nil
}

func (kc *kubectlClient) Delete(manifest string) error {
	//store manifest as file
	manifestFile, err := kc.getManifestFile(manifest)
	if err != nil {
		return err
	}
	//call kubectl delete
	args := []string{fmt.Sprintf("--kubeconfig=%s", kc.kubeconfigFile), "delete", "-f", manifestFile}
	_, err = exec.Command(fmt.Sprint(kc.kubecltCmd), args...).CombinedOutput()
	return err
}

func (kc *kubectlClient) Clientset() (*kubernetes.Clientset, error) {
	return (&k8s.ClientBuilder{}).WithFile(kc.kubeconfigFile).Build()
}
