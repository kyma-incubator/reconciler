package compreconciler

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/chart"
	"github.com/kyma-incubator/reconciler/pkg/server"
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

func (r *runner) Run(w http.ResponseWriter, req *http.Request) error {
	model, err := r.model(req)
	if err != nil {
		return err
	}

	kubeClient, err := r.kubeClient(model)
	if err != nil {
		return err
	}

	statusUpdater := newStatusUpdater(r.interval, model.CallbackURL, statusUpdateRetryTimeout, kubeClient.clientSet)
	if err := statusUpdater.start(); err != nil {
		return err
	}

	retryable := func() error {
		err := r.reconcile(kubeClient, model, statusUpdater)
		if err != nil {
			statusUpdater.Failed()
		}
		return err
	}

	err = retry.Do(retryable, retry.Attempts(uint(r.maxRetries)), retry.LastErrorOnly(true))

	if err == nil {
		statusUpdater.Success()
	} else {
		statusUpdater.Error()
	}

	return err
}

func (r *runner) model(req *http.Request) (*Reconciliation, error) {
	params := server.NewParams(req)
	contactVersion, err := params.String(paramContractVersion)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		return nil, err
	}

	var model = r.modelForVersion(contactVersion)
	err = json.Unmarshal(b, model)
	if err != nil {
		return nil, err
	}

	return model, err
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

func (r *runner) reconcile(kubeClient *kubeClient, model *Reconciliation, statusUpdater *StatusUpdater) error {
	if r.preInstallAction != nil {
		if err := r.preInstallAction.Run(model.Version, kubeClient.clientSet); err != nil {
			return err
		}
	}
	if r.installAction == nil {
		if err := r.install(model, kubeClient, statusUpdater); err != nil {
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

func (r *runner) modelForVersion(contactVersion string) *Reconciliation {
	return &Reconciliation{} //change this function if different contract versions have to be supported
}

func (r *runner) install(model *Reconciliation, client *kubeClient, statusUpdater *StatusUpdater) error {
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
	if err != nil {
		statusUpdater.status = Failed
		return err
	}

	args2 := []string{command, "get", "-f", manifestPath, fmt.Sprintf("--kubeconfig=%s", client.kubeConfigPath), "-oyaml", "-o=jsonpath='{.items[*].metadata.name} {.items[*].metadata.namespace} {.items[*].kind}'"}
	stout, err := exec.Command(args2[0], args2[1:]...).CombinedOutput()
	if err != nil {
		return err
	}
	split := strings.Split(strings.TrimSuffix(string(stout), "'"), " ")
	quantityObjects := len(split) / 3
	statusUpdater.createdObjects = make([]K8SObject, 0, quantityObjects)
	for i := 0; i < quantityObjects; i++ {
		statusUpdater.createdObjects = append(statusUpdater.createdObjects, K8SObject{
			Name:      split[i],
			Namespace: split[i+quantityObjects],
			Kind:      split[i+(2*quantityObjects)],
		})
	}
	return nil

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
