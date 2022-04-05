package cleanup

import (
	"github.com/kyma-incubator/reconciler/pkg/model"
	"time"

	"go.uber.org/zap"
	apixv1beta1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultHTTPTimeout = 30 * time.Second //Expose as a configuration option if necessary
	namespaceTimeout   = 6 * time.Minute  //Expose as a configuration option if necessary
	crLabelReconciler  = "reconciler.kyma-project.io/managed-by=reconciler"
	crLabelIstio       = "install.operator.istio.io/owning-resource-namespace=istio-system"
	kymaNamespace      = "kyma-system"
)

//KymaCRDsFinder returns a list of all CRDs defined explicitly in Kyma sources/charts.
type KymaCRDsFinder func() ([]schema.GroupVersionResource, error)

//Implements cleanup logic
type CliCleaner struct {
	k8s                  KymaKube
	apixClient           apixv1beta1client.ApiextensionsV1beta1Interface
	keepCRDs             bool
	dropKymaCRFinalizers bool
	kymaCRDsFinder       KymaCRDsFinder
	namespaces           []string
	namespaceTimeout     time.Duration
	logger               *zap.SugaredLogger
	dropKymaNamespaces   bool
}

func NewCliCleaner(kubeconfigData string, namespaces []string, logger *zap.SugaredLogger, taskConfig map[string]interface{}, crdsFinder KymaCRDsFinder) (*CliCleaner, error) {

	kymaKube, err := NewFromConfigWithTimeout(kubeconfigData, defaultHTTPTimeout)
	if err != nil {
		return nil, err
	}

	var apixClient *apixv1beta1client.ApiextensionsV1beta1Client
	if apixClient, err = apixv1beta1client.NewForConfig(kymaKube.RestConfig()); err != nil {
		return nil, err
	}

	// check if CRs should be dropped - should be the first step of cleanup
	dropKymaCRFinalizers := readTaskConfigValue(taskConfig, model.DeleteStrategyKey) != "all"
	if dropKymaCRFinalizers {
		dropKymaCRFinalizers = readTaskConfigValue(taskConfig, model.CleanerTypeKey) == model.CleanerCr
	}

	// check if namespaces should be dropped - should be the last step of cleanup
	dropKymaNamespaces := readTaskConfigValue(taskConfig, model.CleanerTypeKey) == model.CleanerNamespace
	return &CliCleaner{kymaKube, apixClient, true, dropKymaCRFinalizers, crdsFinder, namespaces, namespaceTimeout, logger, dropKymaNamespaces}, nil
}

//Run runs the command
func (cmd *CliCleaner) Run() error {
	if err := cmd.deletePVCSAndWait(kymaNamespace); err != nil {
		return err
	}
	if err := cmd.removeResourcesFinalizers(); err != nil {
		return err
	}
	if err := cmd.deleteKymaNamespaces(); err != nil {
		return err
	}
	if err := cmd.waitForNamespaces(); err != nil {
		return err
	}
	return nil
}

func contains(items []string, item string) bool {
	for _, i := range items {
		if i == item {
			return true
		}
	}
	return false
}

//taken from github.com/kyma-project/cli/internal/kube/kube.go
type KymaKube interface {
	Static() kubernetes.Interface
	Dynamic() dynamic.Interface

	// RestConfig provides the REST configuration of the kubernetes client
	RestConfig() *rest.Config
}

// NewFromConfigWithTimeout creates a new Kubernetes client based on the given Kubeconfig provided by a file (out-of-cluster config).
// Allows to set a custom timeout for the Kubernetes HTTP client.
func NewFromConfigWithTimeout(kubeconfigData string, t time.Duration) (KymaKube, error) {
	config, err := restConfig(kubeconfigData)
	if err != nil {
		return nil, err
	}

	config.Timeout = t

	sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client{
		static:  sClient,
		dynamic: dClient,
		restCfg: config,
	}, nil
}

//taken from github.com/kyma-project/cli/internal/kube/client.go
//client is the default KymaKube implementation
type client struct {
	static  kubernetes.Interface
	dynamic dynamic.Interface
	restCfg *rest.Config
}

func (c *client) Static() kubernetes.Interface {
	return c.static
}

func (c *client) Dynamic() dynamic.Interface {
	return c.dynamic
}

func (c *client) RestConfig() *rest.Config {
	return c.restCfg
}

// restConfig loads the rest configuration needed by k8s clients to interact with clusters based on the kubeconfig.
// Loading rules are based on standard defined kubernetes config loading.
func restConfig(kubeconfigData string) (*rest.Config, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfigData))
	if err != nil {
		return nil, err
	}
	cfg.WarningHandler = rest.NoWarnings{}
	return cfg, nil
}

func readTaskConfigValue(config map[string]interface{}, key string) string {
	v := config[key]
	if v == nil {
		return ""
	}

	s, ok := v.(string)
	if !ok {
		return ""
	}

	return s
}
