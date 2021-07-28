package kubernetes

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

type kubectlClient struct {
	kubectlCmd     string
	kubeconfigFile string
	manifestFile   string
}

func newKubectlClient(kubeconfig string) (Client, error) {
	kubectlCmd, err := kubectl()
	if err != nil {
		return nil, err
	}

	kubeconfigFile := "/tmp/kubeconfig-" + uuid.New().String()
	if err := ioutil.WriteFile(kubeconfigFile, []byte(kubeconfig), 0600); err != nil {
		return nil, err
	}

	return &kubectlClient{
		kubectlCmd:     kubectlCmd,
		kubeconfigFile: kubeconfigFile,
	}, nil
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
			return "", errors.Wrap(err, "failed to store manifests in a file")
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
	args := []string{fmt.Sprintf("--kubeconfig=%s", kc.kubeconfigFile), "apply", "-f", manifestFile, "--validate=false"} // TODO remove --validate=false after fix issue  https://github.com/kyma-project/kyma/issues/11738
	//nolint:gosec //arguments for cmd call not allowed: replace command-call with Go k8s-client
	output, err := exec.Command(kc.kubectlCmd, args...).CombinedOutput()
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Exeuction of kubeclt command with args '%s' failed: %s", strings.Join(args, " "), output))
	}
	return err
}

func (kc *kubectlClient) DeployedResources(manifest string) ([]Resource, error) {
	manifestFile, err := kc.getManifestFile(manifest)
	if err != nil {
		return nil, err
	}

	args := []string{"get", "-f", manifestFile, fmt.Sprintf("--kubeconfig=%s", kc.kubeconfigFile), "-ojson"}
	//nolint:gosec //arguments for cmd call not allowed: replace command-call with Go k8s-client
	getCommandStout, err := exec.Command(kc.kubectlCmd, args...).CombinedOutput()
	if err != nil {
		return nil, err
	}
	resourcesText := string(getCommandStout)
	if strings.HasPrefix(resourcesText, "Warning") {
		index := strings.Index(resourcesText, "{")
		getCommandStout = []byte(resourcesText[index:])
	}
	//marshal json result
	resources := make(map[string]interface{})
	err = json.Unmarshal(getCommandStout, &resources)
	if err != nil {
		return nil, err
	}

	//extract resources
	var result []Resource
	for _, item := range resources["items"].([]interface{}) {
		res := item.(map[string]interface{})
		metadata := res["metadata"].(map[string]interface{})
		namespace, ok := metadata["namespace"]
		if !ok {
			namespace = ""
		}
		result = append(result, Resource{
			Kind:      res["kind"].(string),
			Name:      metadata["name"].(string),
			Namespace: namespace.(string),
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
	//nolint:gosec //arguments for cmd call not allowed: replace command-call with Go k8s-client
	_, err = exec.Command(kc.kubectlCmd, args...).CombinedOutput()
	return err
}

func (kc *kubectlClient) Clientset() (*kubernetes.Clientset, error) {
	return (&k8s.ClientBuilder{}).WithFile(kc.kubeconfigFile).Build()
}
