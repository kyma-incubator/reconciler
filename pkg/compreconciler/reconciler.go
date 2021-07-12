package compreconciler

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"k8s.io/client-go/dynamic"
)

const (
	paramContractVersion = "version"
	maxRetries           = 5
)

type Action interface {
	Run(version string, kubeClient *dynamic.Interface, status *StatusUpdater) error
}

type ComponentReconciler struct {
	kubeClient *dynamic.Interface
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
			}).run(w, req)
		}).
		Methods("PUT", "POST")
	server := server.Webserver{
		Router: router,
	}
	return server.Start()
}
