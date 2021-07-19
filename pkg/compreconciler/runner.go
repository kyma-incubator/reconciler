package compreconciler

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/chart"
	k8s "github.com/kyma-incubator/reconciler/pkg/kubernetes"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"os"
	"os/exec"
	"strings"
)

const (
	envVarKubectlPath = "KUBECTL_PATH"
)

type runner struct {
	*ComponentReconciler
}

//TODO: drop this struct after switching to Go k8s-client.. clientSet should be used directly!
type kubeClient struct {
	clientSet      *kubernetes.Clientset
	kubeConfigPath string
}

func (r *runner) Run(ctx context.Context, model *Reconciliation, callback CallbackHandler) error {
	statusUpdater := newStatusUpdater(ctx, r.updateInterval, callback, uint(r.maxRetries), r.debug)

	retryable := func(statusUpdater *StatusUpdater) func() error {
		return func() error {
			if err := statusUpdater.Running(); err != nil {
				return err
			}
			err := r.reconcile(model)
			if err != nil {
				if err := statusUpdater.Failed(); err != nil {
					return err
				}
			}
			return err
		}
	}(statusUpdater)

	//retry the reconciliation in case of an error
	err := retry.Do(retryable,
		retry.Attempts(uint(r.maxRetries)),
		retry.Delay(r.retryDelay),
		retry.LastErrorOnly(false),
		retry.Context(ctx))

	logger := r.logger()
	if err == nil {
		logger.Info(
			fmt.Sprintf("Reconciliation of component '%s' for version '%s' finished successfully",
				model.Component, model.Version))
		if err := statusUpdater.Success(); err != nil {
			return err
		}
	} else {
		logger.Warn(
			fmt.Sprintf("Retryable reconciliation of component '%s' for version '%s' failed consistently: giving up",
				model.Component, model.Version))
		if err := statusUpdater.Error(); err != nil {
			return err
		}
	}

	return err
}

func (r *runner) kubeClient(model *Reconciliation) (*kubeClient, error) {
	if model.Kubeconfig == "" {
		return nil, fmt.Errorf("kubeconfig is missing in reconciliation model")
	}

	clientSet, err := (&k8s.ClientBuilder{}).WithString(model.Kubeconfig).Build()
	if err != nil {
		return nil, err
	}

	//TODO: drop this block after switching to K8s Go-client.. only clientSet should be returned!
	kubeConfigPath := "/tmp/kubeconfig-" + uuid.New().String()
	if err := ioutil.WriteFile(kubeConfigPath, []byte(model.Kubeconfig), 0600); err != nil {
		return nil, err
	}

	return &kubeClient{
		clientSet:      clientSet,
		kubeConfigPath: kubeConfigPath,
	}, nil
}

func (r *runner) reconcile(model *Reconciliation) error {
	kubeClient, err := r.kubeClient(model)
	if err != nil {
		return err
	}

	logger := r.logger()
	if r.preInstallAction != nil {
		if err := r.preInstallAction.Run(model.Version, kubeClient.clientSet); err != nil {
			logger.Warn(
				fmt.Sprintf("Pre-installation action of version '%s' failed: %s", model.Version, err))
			return err
		}
	}

	if r.installAction == nil {
		if err := r.install(model, kubeClient); err != nil {
			logger.Warn(
				fmt.Sprintf("Default-installation of version '%s' failed: %s", model.Version, err))
			return err
		}
	} else {
		if err := r.installAction.Run(model.Version, kubeClient.clientSet); err != nil {
			logger.Warn(
				fmt.Sprintf("Installation action of version '%s' failed: %s", model.Version, err))
			return err
		}
	}

	if r.postInstallAction != nil {
		if err := r.postInstallAction.Run(model.Version, kubeClient.clientSet); err != nil {
			logger.Warn(
				fmt.Sprintf("Post-installation action of version '%s' failed: %s", model.Version, err))
			return err
		}
	}

	return nil
}

func (r *runner) install(model *Reconciliation, client *kubeClient) error {
	command, err := r.kubectlCmd()
	if err != nil {
		return err
	}

	manifestFile, err := r.renderManifestFile(model) //TODO: use manifest-string directly instead of a file
	if err != nil {
		return err
	}

	if err := r.deployManifest(client, command, manifestFile); err != nil {
		return err
	}
	return r.trackProgress(client, command, manifestFile) //blocking call
}

func (r *runner) renderManifestFile(model *Reconciliation) (string, error) {
	manifests, err := r.chartProvider.Manifests(r.newComponentSet(model), &chart.Options{})
	if err != nil {
		return "", err
	}

	if len(manifests) != 1 { //just an assertion - can in current implementation not occur
		return "", fmt.Errorf("reconciliation can only process 1 manifest but got %d", len(manifests))
	}

	//TODO: drop this block after switching to Go k8s-client... manifest-string should be returned instead!
	manifestFile := "/tmp/manifest-" + uuid.New().String()
	if err := ioutil.WriteFile(manifestFile, []byte(manifests[0].Manifest), 0600); err != nil {
		return "", err
	}

	return manifestFile, nil
}

//TODO: drop me after switching to Go ks8-client
func (r *runner) kubectlCmd() (string, error) {
	command, ok := os.LookupEnv(envVarKubectlPath)
	if !ok {
		return "", fmt.Errorf("cannot find kubectl cmd, please set env-var '%s'", envVarKubectlPath)
	}
	return command, nil
}

//TODO: refactor this method after switching to Go k8s-client: deploy manifest string using Go k8s-client
func (r *runner) deployManifest(client *kubeClient, command, manifestFile string) error {
	args := []string{fmt.Sprintf("--kubeconfig=%s", client.kubeConfigPath), "apply", "-f", manifestFile}
	_, err := exec.Command(command, args...).CombinedOutput()
	return err
}

func (r *runner) trackProgress(client *kubeClient, command, manifestFile string) error {
	//get resources defined in manifest
	args := []string{"get", fmt.Sprintf("-f %s", manifestFile), fmt.Sprintf("--kubeconfig=%s", client.kubeConfigPath), "-oyaml", "-o=jsonpath='{.items[*].metadata.name} {.items[*].metadata.namespace} {.items[*].kind}'"}
	getCommandStout, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		return err
	}

	//convert resource-string to k8sObjects
	split := strings.Split(strings.TrimSuffix(string(getCommandStout), "'"), " ")
	quantityObjects := len(split) / 3

	pt, err := NewProgressTracker(client.clientSet, r.debug, ProgressTrackerConfig{})
	if err != nil {
		return err
	}
	//register Kubernetes resources the progress tracker has to watch
	for i := 0; i < quantityObjects; i++ {
		watchable, err := NewWatchableResource(split[i+(2*quantityObjects)]) //convert "kind" to watchable
		if err != nil {
			return err
		}
		pt.AddResource(
			watchable,
			split[i+quantityObjects], //namespace
			split[i],                 //name
		)
	}
	return pt.Watch() //blocking call
}

func (r *runner) newComponentSet(model *Reconciliation) *chart.ComponentSet {
	comp := chart.NewComponent(model.Component, model.Namespace, r.configMap(model))
	compSet := chart.NewComponentSet(model.Kubeconfig, model.Version, model.Profile, []*chart.Component{comp})
	return compSet
}

func (r *runner) configMap(model *Reconciliation) map[string]interface{} {
	result := make(map[string]interface{}, len(model.Configuration))
	for _, comp := range model.Configuration {
		result[comp.Key] = comp.Value
	}
	return result
}
