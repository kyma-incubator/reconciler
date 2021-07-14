package compreconciler

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	envVarKubectlPath = "KUBECTL_PATH"
)

type runner struct {
	preInstallAction  Action
	installAction     Action
	postInstallAction Action
	maxRetries        int
	interval          time.Duration
	debug             bool
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

	statusUpdater := newStatusUpdater(int(r.interval.Seconds()), model.CallbackURL, r.maxRetries)
	statusUpdater.start()
	defer statusUpdater.stop()

	//run in goroutine:
	if err := r.reconcile(model, statusUpdater); err != nil {
		statusUpdater.failed()
		return err //replace with retry
	}

	return nil
}

func (r *runner) model(req *http.Request) (*ReconciliationModel, error) {
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

func (r *runner) kubeClient(model *ReconciliationModel) (*kubeClient, error) {
	kubeConfigPath := "/tmp/kubeconfig-" + uuid.New().String()
	if err := ioutil.WriteFile(kubeConfigPath, []byte(model.KubeConfig), 0600); err != nil {
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

func (r *runner) reconcile(model *ReconciliationModel, statusUpdater *StatusUpdater) error {
	kubeClient, err := r.kubeClient(model)
	if err != nil {
		return err
	}

	if r.preInstallAction != nil {
		if err := r.preInstallAction.Run(model.Version, kubeClient.clientSet, statusUpdater); err != nil {
			return err
		}
	}
	if r.installAction == nil {
		if err := r.install(model, kubeClient); err != nil {
			return err
		}
	} else {
		if err := r.installAction.Run(model.Version, kubeClient.clientSet, statusUpdater); err != nil {
			return err
		}
	}

	if r.postInstallAction != nil {
		if err := r.postInstallAction.Run(model.Version, kubeClient.clientSet, statusUpdater); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) modelForVersion(contactVersion string) *ReconciliationModel {
	return &ReconciliationModel{} //change this function if different contract versions have to be supported
}

func (r *runner) install(model *ReconciliationModel, client *kubeClient) error {
	manifestPath := "/tmp/manifest-" + uuid.New().String()
	if err := ioutil.WriteFile(manifestPath, []byte(model.Manifest), 0600); err != nil {
		return err
	}

	command, ok := os.LookupEnv(envVarKubectlPath)
	if !ok {
		return fmt.Errorf("Cannot find kubectl cmd, please set env-var '%s'", envVarKubectlPath)
	}
	args := []string{fmt.Sprintf("--kubeconfig=%s", client.kubeConfigPath), "apply", "-f", manifestPath}
	_, err := exec.Command(command, args...).CombinedOutput()
	return err
}
