package compreconciler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

type Run struct {
	*ComponentReconciler
	preInstallAction  Action
	postInstallAction Action
	maxRetries        int
}

func (crr *Run) run(w http.ResponseWriter, r *http.Request) error {
	//TODO: consider contrac tversion when choosing model
	//params := server.NewParams(r)
	//contactVersion, err := params.String(paramContractVersion)
	// if err != nil {
	// 	return err
	// }

	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		return err
	}

	// Unmarshal
	var reconModel = &ReconciliationModel{} //please consider contactVersion to decide which model has to be used
	err = json.Unmarshal(b, reconModel)
	if err != nil {
		return err
	}

	//trigger reconciliation
	statusUpdater := newStatusUpdater(30*time.Second, reconModel.CallbackURL, crr.maxRetries) //TODO: make interval configurable
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

func (r *Run) apply(model *ReconciliationModel) error {
	//https://github.com/billiford/go-clouddriver/blob/master/pkg/kubernetes/client.go
	//TODO: implement installation logic
	return nil
}
