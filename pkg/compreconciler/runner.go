package compreconciler

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/avast/retry-go"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/chart"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	envVarKubectlPath        = "KUBECTL_PATH"
	statusUpdateRetryTimeout = 30 * time.Minute
)

type runner struct {
	*ComponentReconciler
}

type kubeClient struct {
	clientSet      *kubernetes.Clientset
	kubeConfigPath string
}

func (r *runner) Run(ctx context.Context, model *Reconciliation, statusUpdater *StatusUpdater) error {
	if err := statusUpdater.Start(); err != nil {
		return err
	}

	kubeClient, err := r.kubeClient(model)
	if err != nil {
		return err
	}

	retryable := func() error {
		err := r.reconcile(kubeClient, model)
		if err != nil {
			statusUpdater.Failed()
		}
		return err
	}

	//retry the reconciliation in case of an error
	err = retry.Do(retryable,
		retry.Attempts(uint(r.maxRetries)),
		retry.LastErrorOnly(true),
		retry.Context(ctx))

	if err == nil {
		statusUpdater.Success()
	} else {
		statusUpdater.Error()
	}

	return err
}

func (r *runner) kubeClient(model *Reconciliation) (*kubeClient, error) {
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

func (r *runner) reconcile(kubeClient *kubeClient, model *Reconciliation) error {
	if r.preInstallAction != nil {
		if err := r.preInstallAction.Run(model.Version, kubeClient.clientSet); err != nil {
			return err
		}
	}
	if r.installAction == nil {
		if err := r.install(model, kubeClient); err != nil {
			return err
		}
	} else {
		if err := r.installAction.Run(model.Version, kubeClient.clientSet); err != nil {
			return err
		}
	}

	if r.postInstallAction != nil {
		if err := r.postInstallAction.Run(model.Version, kubeClient.clientSet); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) install(model *Reconciliation, client *kubeClient) error {
	manifests, err := r.chartProvider.Manifests(r.newChartComponentSet(model), &chart.Options{})
	if err != nil {
		return err
	}

	if len(manifests) != 1 { //just an assertion - can in current implementation not occur
		return fmt.Errorf("Reconciliation can only process 1 manifest but got %d", len(manifests))
	}

	manifestPath := "/tmp/manifest-" + uuid.New().String()
	if err := ioutil.WriteFile(manifestPath, []byte(manifests[0].Manifest), 0600); err != nil {
		return err
	}

	command, ok := os.LookupEnv(envVarKubectlPath)
	if !ok {
		return fmt.Errorf("Cannot find kubectl cmd, please set env-var '%s'", envVarKubectlPath)
	}
	args := []string{fmt.Sprintf("--kubeconfig=%s", client.kubeConfigPath), "apply", "-f", manifestPath}
	_, err = exec.Command(command, args...).CombinedOutput()
	return err
}

func (r *runner) newChartComponentSet(model *Reconciliation) *chart.ComponentSet {
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
