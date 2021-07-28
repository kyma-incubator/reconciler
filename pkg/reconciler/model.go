package reconciler

import (
	"fmt"
)

//Reconciliation is the model for incoming reconciliation requests
type Reconciliation struct {
	ComponentsReady []string        `json:"componentsReady"`
	Component       string          `json:"component"`
	Namespace       string          `json:"namespace"`
	Version         string          `json:"version"`
	Profile         string          `json:"profile"`
	Configuration   []Configuration `json:"configuration"`
	Kubeconfig      string          `json:"kubeconfig"`
	//CallbackURL is mandatory when component-reconciler runs in separate process
	CallbackURL string `json:"callbackURL"`
	//CallbackFct has to be set when component-reconciler runs embedded
	CallbackFct func(status Status) error `json:"-"`
	//Flag to indicate that CRDs have to be installed
	InstallCRD bool
}

func (r *Reconciliation) String() string {
	return fmt.Sprintf("Reconciliation [Component:%s,Version:%s,Namespace:%s,Profile:%s]",
		r.Component, r.Version, r.Namespace, r.Profile)
}

type Configuration struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Status string

const (
	NotStarted Status = "notstarted"
	Failed     Status = "failed"
	Error      Status = "error"
	Running    Status = "running"
	Success    Status = "success"
)

//HTTPErrorResponse is the model used for general error responses
type HTTPErrorResponse struct {
	Error error
}

//HTTPMissingDependenciesResponse is the model used for missing dependency responses
type HTTPMissingDependenciesResponse struct {
	Dependencies struct {
		Required []string
		Missing  []string
	}
}
