package compreconciler

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/server"
)

const (
	paramContractVersion        = "version"
	maxRetries                  = 5
	intervalReconciliationInSec = 5 // TODO before merge, change to 30
)

type Action interface {
	Run(version string, kubeClient *kubernetes.Clientset, status *StatusUpdater) error
}

type ComponentReconciler struct {
	kubeClient *kubernetes.Clientset
	manifest   string
}

func NewComponentReconciler() *ComponentReconciler {
	return &ComponentReconciler{}
}

func (r *ComponentReconciler) Reconcile(preInstallAction, postInstallAction Action) error {
	//create webserver
	router := mux.NewRouter()
	router.HandleFunc(
		fmt.Sprintf("/v{%s}/run", paramContractVersion),
		func(w http.ResponseWriter, req *http.Request) {
			(&Run{
				r,
				preInstallAction,
				postInstallAction,
				maxRetries,
				intervalReconciliationInSec,
			}).run(w, req)
		}).
		Methods("PUT", "POST")
	server := server.Webserver{
		Port:   8080,
		Router: router,
	}
	return server.Start()
}
