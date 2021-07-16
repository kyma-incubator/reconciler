package compreconciler

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/chart"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
	kubeConfigPath := "/tmp/kubeconfig-" + uuid.New().String()
	if err := ioutil.WriteFile(kubeConfigPath, []byte(model.Kubeconfig), 0600); err != nil {
		return nil, err
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
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

	manifestFile, err := r.renderManifestFile(model)
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

	manifestFile := "/tmp/manifest-" + uuid.New().String()
	if err := ioutil.WriteFile(manifestFile, []byte(manifests[0].Manifest), 0600); err != nil {
		return "", err
	}

	return manifestFile, nil
}

func (r *runner) kubectlCmd() (string, error) {
	command, ok := os.LookupEnv(envVarKubectlPath)
	if !ok {
		return "", fmt.Errorf("cannot find kubectl cmd, please set env-var '%s'", envVarKubectlPath)
	}
	return command, nil
}

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

	k8sObjects := make([]*k8sObject, 0, quantityObjects)
	for i := 0; i < quantityObjects; i++ {
		k8sObjects = append(k8sObjects, &k8sObject{
			name:      split[i],
			namespace: split[i+quantityObjects],
			kind:      split[i+(2*quantityObjects)],
		})
	}

	//wait until all resources are deployed (or timeout is reached)
	newProgressTracker(k8sObjects, client.clientSet, r.debug) //TODO: block until progress is ready OR timeout reached
	return nil
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
