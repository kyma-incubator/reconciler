package actions

import (
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"os"
)

const (
	istioctlBinaryPathEnvKey = "ISTIOCTL_PATH"
	istioOperatorKind        = "IstioOperator"
)

type webhookPatchJSON struct {
	Op    string                `json:"op"`
	Path  string                `json:"path"`
	Value webhookPatchJSONValue `json:"value"`
}

type webhookPatchJSONValue struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

//go:generate mockery -name=IstioPerformer
// IstioPerformer performs actions on Istio component on the cluster.
type IstioPerformer interface {

	// Install Istio on the cluster.
	Install(kubeConfig, manifest string, logger *zap.SugaredLogger, commander istioctl.Commander) error

	// PatchMutatingWebhook configuration.
	PatchMutatingWebhook(kubeClient kubernetes.Client, logger *zap.SugaredLogger) error

	// Update Istio on the cluster.
	Update(kubeConfig, manifest string, logger *zap.SugaredLogger, commander istioctl.Commander) error
}

// DefaultIstioPerformer provides a default implementation of IstioPerformer.
type DefaultIstioPerformer struct {
	commander istioctl.Commander
}

// NewDefaultIstioPerformer creates a new instance of the DefaultIstioPerformer.
func NewDefaultIstioPerformer(commander istioctl.Commander) *DefaultIstioPerformer {
	return &DefaultIstioPerformer{
		commander: commander,
	}
}

func (c *DefaultIstioPerformer) Install(kubeConfig, manifest string, logger *zap.SugaredLogger, commander istioctl.Commander) error {
	istioctlPath, err := resolveIstioctlPath()
	if err != nil {
		return err
	}

	istioOperator, err := extractIstioOperatorContextFrom(manifest)
	if err != nil {
		return err
	}

	istioOperatorPath, istioOperatorCf, err := file.CreateTempFileWith(istioOperator)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := istioOperatorCf()
		if cleanupErr != nil {
			logger.Error(cleanupErr)
		}
	}()

	kubeconfigPath, kubeconfigCf, err := file.CreateTempFileWith(kubeConfig)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			logger.Error(cleanupErr)
		}
	}()

	logger.Info("Starting Istio installation...")

	err = commander.Install(istioctlPath, istioOperatorPath, kubeconfigPath)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	return nil
}

func (c *DefaultIstioPerformer) PatchMutatingWebhook(kubeClient kubernetes.Client, logger *zap.SugaredLogger) error {
	patchContent := []webhookPatchJSON{{
		Op:   "add",
		Path: "/webhooks/4/namespaceSelector/matchExpressions/-",
		Value: webhookPatchJSONValue{
			Key:      "gardener.cloud/purpose",
			Operator: "NotIn",
			Values: []string{
				"kube-system",
			},
		},
	}}

	patchContentJSON, err := json.Marshal(patchContent)
	if err != nil {
		return err
	}

	logger.Info("Patching istio-sidecar-injector MutatingWebhookConfiguration...")

	err = kubeClient.PatchUsingStrategy("MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", patchContentJSON, types.JSONPatchType)
	if err != nil {
		return err
	}

	logger.Infof("Patch has been applied successfully")

	return nil
}

func (c *DefaultIstioPerformer) Update(kubeConfig, manifest string, logger *zap.SugaredLogger, commander istioctl.Commander) error {
	// TODO: implement upgrade logic, for now let it be error-free
	// Hint: use commander.Upgrade() for binary execution
	return nil
}

func resolveIstioctlPath() (string, error) {
	path := os.Getenv(istioctlBinaryPathEnvKey)
	if path == "" {
		return "", errors.New("Istioctl binary could not be found under ISTIOCTL_PATH env variable")
	}

	return path, nil
}

func extractIstioOperatorContextFrom(manifest string) (string, error) {
	unstructs, err := kubeclient.ToUnstructured([]byte(manifest), true)
	if err != nil {
		return "", err
	}

	for _, unstruct := range unstructs {
		if unstruct.GetKind() != istioOperatorKind {
			continue
		}

		unstructBytes, err := unstruct.MarshalJSON()
		if err != nil {
			return "", nil
		}

		return string(unstructBytes), nil
	}

	return "", errors.New("Istio Operator definition could not be found in manifest")
}
