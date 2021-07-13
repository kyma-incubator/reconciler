package compreconciler

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
	"os/exec"
)

const (
	KubectlPath = "KUBECTL_PATH"
)

type Run struct {
	*ComponentReconciler
	preInstallAction            Action
	postInstallAction           Action
	maxRetries                  int
	intervalReconciliationInSec int
}

func (crr *Run) run(w http.ResponseWriter, r *http.Request) error {
	params := server.NewParams(r)
	contactVersion, err := params.String(paramContractVersion)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		return err
	}

	// Unmarshal
	var reconModel = reconciliationModelForVersion(contactVersion)
	err = json.Unmarshal(b, reconModel)
	if err != nil {
		return err
	}

	// TODO render menifest by ChartProvider

	kubeConfigPath := "/tmp/kubeconfig-" + uuid.New().String()
	manifestPath := "/tmp/manifest-" + uuid.New().String()
	if err := ioutil.WriteFile(kubeConfigPath, []byte(reconModel.KubeConfig), 0644); err != nil {
		log.Println(err)
		return err
	}
	if err := ioutil.WriteFile(manifestPath, []byte(reconModel.Manifest), 0644); err != nil {
		log.Println(err)
		return err
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		log.Println(err)
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println(err)
		return err
	}
	crr.kubeClient = clientset

	//trigger reconciliation
	statusUpdater := newStatusUpdater(intervalReconciliationInSec, reconModel.CallbackURL, crr.maxRetries)
	statusUpdater.start()
	if crr.preInstallAction != nil {
		if err := crr.preInstallAction.Run(reconModel.Version, crr.kubeClient, statusUpdater); err != nil {
			statusUpdater.failed()
			return err
		}
	}
	if err := crr.apply(kubeConfigPath, manifestPath); err != nil {
		statusUpdater.failed()
		return err
	}
	if crr.postInstallAction != nil {
		if err := crr.postInstallAction.Run(reconModel.Version, crr.kubeClient, statusUpdater); err != nil {
			statusUpdater.failed()
			return err
		}
	}
	return nil
}

func reconciliationModelForVersion(contactVersion string) *ReconciliationModel {
	return &ReconciliationModel{}
}

func (r *Run) apply(kubeConfigPath, manifest string) error {
	command := "kubectl"
	env, ok := os.LookupEnv(KubectlPath)
	if ok {
		command = env
	}
	args := []string{command, "apply", "-f", manifest}
	args = append(args, fmt.Sprintf("--kubeconfig=%s", kubeConfigPath))
	stout, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		log.Println(err)
		return err
	}
	log.Println("kubectl apply output: " + string(stout))
	return nil
}
