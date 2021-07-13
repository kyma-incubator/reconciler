package compreconciler

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"io/ioutil"
	"net/http"
	"os/exec"
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

	//trigger reconciliation
	statusUpdater := newStatusUpdater(intervalReconciliationInSec, reconModel.CallbackURL, crr.maxRetries)
	statusUpdater.start()
	if crr.preInstallAction != nil {
		if err := crr.preInstallAction.Run(reconModel.Version, crr.kubeClient, statusUpdater); err != nil {
			statusUpdater.failed()
			return err
		}
	}
	if err := crr.apply(reconModel); err != nil {
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

func (r *Run) apply(model *ReconciliationModel) error {
	name := uuid.New()
	if err := ioutil.WriteFile("kubeconfig-"+name.String(), []byte(model.KubeConfig), 0644); err != nil {
		return err
	}
	if err := ioutil.WriteFile("manifest-"+name.String(), []byte(model.Manifest), 0644); err != nil {
		return err
	}
	args := []string{"./kubectl", "apply", "-f", "manifest-" + name.String()}
	args = append(args, fmt.Sprintf("--kubeconfig=%s", "kubeconfig-"+name.String()))
	_, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}
