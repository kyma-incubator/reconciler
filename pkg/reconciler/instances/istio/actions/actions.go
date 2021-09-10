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
	istioOperatorKind = "IstioOperator"
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
	Install() error

	// PatchMutatingWebhook configuration.
	PatchMutatingWebhook() error
}

// DefaultIstioPerformer provides a default implementation of IstioPerformer.
type DefaultIstioPerformer struct {
	commander     istioctl.Commander
	istioctlPath  string
	kubeConfig    string
	istioOperator string
	kubeClient    kubernetes.Client
	logger        *zap.SugaredLogger
}

// NewDefaultIstioPerformer creates a new instance of the DefaultIstioPerformer.
func NewDefaultIstioPerformer(kubeConfig, manifest string, kubeClient kubernetes.Client, logger *zap.SugaredLogger, cmder istioctl.Commander) (*DefaultIstioPerformer, error) {
	istioctlPath, err := resolveIstioctlPath()
	if err != nil {
		return nil, err
	}

	istioOperator, err := extractIstioOperatorContextFrom(manifest)
	if err != nil {
		return nil, err
	}

	return &DefaultIstioPerformer{
		istioctlPath:  istioctlPath,
		kubeConfig:    kubeConfig,
		istioOperator: istioOperator,
		kubeClient:    kubeClient,
		logger:        logger,
		commander:     cmder,
	}, nil
}

func (c *DefaultIstioPerformer) Install() error {
	istioOperatorPath, istioOperatorCf, err := file.CreateTempFileWith(c.istioOperator)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := istioOperatorCf()
		if cleanupErr != nil {
			c.logger.Error(cleanupErr)
		}
	}()

	kubeconfigPath, kubeconfigCf, err := file.CreateTempFileWith(c.kubeConfig)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			c.logger.Error(cleanupErr)
		}
	}()

	c.logger.Info("Starting Istio installation...")

	err = c.commander.Install(c.istioctlPath, istioOperatorPath, kubeconfigPath)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	return nil
}

func (c *DefaultIstioPerformer) PatchMutatingWebhook() error {
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

	c.logger.Info("Patching istio-sidecar-injector MutatingWebhookConfiguration...")

	err = c.kubeClient.PatchUsingStrategy("MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", patchContentJSON, types.JSONPatchType)
	if err != nil {
		return err
	}

	c.logger.Infof("Patch has been applied successfully")

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
