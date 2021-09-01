package reconciler

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
	"net/url"
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
	Repository      Repository      `json:"repository"`

	//These fields are not part of HTTP request coming from reconciler-controller:
	CallbackFct func(status Status) error `json:"-"` //CallbackFct is mandatory when component-reconciler runs embedded in another process
}

func (r *Reconciliation) ConfigsToMap() map[string]string {
	configs := make(map[string]string)
	for i := 0; i < len(r.Configuration); i++ {
		configs[r.Configuration[i].Key] = r.Configuration[i].Value
	}
	return configs
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

type MothershipReconcilerConfig struct {
	Host          string
	Port          int
	CrdComponents []string
	PreComponents []string
}

type Repository struct {
	URL   string `json:"url"`
	Token string `json:"-"`
}

func (r *Repository) ReadToken(clientSet core.CoreV1Interface, namespace string) error {
	secretKey, err := MapSecretKey(r.URL)
	if err != nil {
		return err
	}

	secret, err := clientSet.
		Secrets(namespace).
		Get(context.Background(), secretKey, v1.GetOptions{})

	if err != nil {
		return err
	}

	r.Token = strings.Trim(string(secret.Data["token"]), "\n")

	return nil
}

func MapSecretKey(URL string) (string, error) {
	parsed, err := url.Parse(URL)

	URL = strings.ReplaceAll(URL, "www.", "")

	if !strings.HasPrefix(URL, "http") {
		return "", errors.Errorf("Invalid URL configuration")
	}

	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" {
		return parsed.Path, nil
	}

	output := strings.ReplaceAll(parsed.Host, ":"+parsed.Port(), "")
	output = strings.ReplaceAll(output, "www.", "")

	return output, nil
}

func (r *Repository) String() string {
	return r.URL
}
