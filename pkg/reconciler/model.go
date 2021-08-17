package reconciler

import (
	"fmt"
	"strings"
)

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

const (
	ManagedByLabel       = "reconciler.kyma-project.io/managed-by"
	LabelReconcilerValue = "reconciler"
)

//Reconciliation is the model for reconciliation calls
type Reconciliation struct {
	ComponentsReady []string        `json:"componentsReady"`
	Component       string          `json:"component"`
	Namespace       string          `json:"namespace"`
	Version         string          `json:"version"`
	Profile         string          `json:"profile"`
	Configuration   []Configuration `json:"configuration"`
	Kubeconfig      string          `json:"kubeconfig"`
	CallbackURL     string          `json:"callbackURL"` //CallbackURL is mandatory when component-reconciler runs in separate process
	InstallCRD      bool            `json:"installCRD"`
	CorrelationID   string          `json:"correlationID"`

	//These fields are not part of HTTP request coming from reconciler-controller:
	CallbackFct func(status Status) error `json:"-"` //CallbackFct is mandatory when component-reconciler runs embedded in another process
}

func (r *Reconciliation) String() string {
	return fmt.Sprintf("Reconciliation [Component:%s,Version:%s,Namespace:%s,Profile:%s]",
		r.Component, r.Version, r.Namespace, r.Profile)
}

func (r *Reconciliation) Validate() error {
	//check mandatory fields are defined
	var errFields []string
	r.Component = strings.TrimSpace(r.Component)
	if r.Component == "" {
		errFields = append(errFields, "Component")
	}
	r.Namespace = strings.TrimSpace(r.Namespace)
	if r.Namespace == "" {
		errFields = append(errFields, "Namespace")
	}
	r.Version = strings.TrimSpace(r.Version)
	if r.Version == "" {
		errFields = append(errFields, "Version")
	}
	r.Kubeconfig = strings.TrimSpace(r.Kubeconfig)
	if r.Kubeconfig == "" {
		errFields = append(errFields, "Kubeconfig")
	}
	r.CallbackURL = strings.TrimSpace(r.CallbackURL)
	if r.CallbackFct == nil && r.CallbackURL == "" {
		errFields = append(errFields, "CallbackFct or CallbackURL")
	}
	r.CorrelationID = strings.TrimSpace(r.CorrelationID)
	if r.CorrelationID == "" {
		errFields = append(errFields, "CorrelationID")
	}
	//return aggregated error msg
	var err error
	if len(errFields) > 0 {
		err = fmt.Errorf("mandatory fields are undefined: %s", strings.Join(errFields, ","))
	}
	return err
}

//HTTPErrorResponse is the model used for general error responses
type HTTPErrorResponse struct {
	Error string
}

//HTTPMissingDependenciesResponse is the model used for missing dependency responses
type HTTPMissingDependenciesResponse struct {
	Dependencies struct {
		Required []string
		Missing  []string
	}
}

type CallbackMessage struct {
	Status string `json:"status"`
}

//ComponentReconciler is the model used to describe the component reconciler configuration
type ComponentReconciler struct {
	URL string `json:"url"`
}

type ComponentReconcilersConfig map[string]*ComponentReconciler
