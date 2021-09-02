package istio

import (
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"os/exec"
	"strings"
)

const (
	istioctlBinaryPathEnvKey = "ISTIOCTL_PATH"
	yamlDelimiter            = "---"
	istioOperatorKind        = "kind: IstioOperator"
	istioNamespace           = "istio-system"
 	istioChart               = "istio-configuration"
)

type webhookPatchJson struct {
	Op string `json:"op"`
	Path string `json:"path"`
	Value webhookPatchJsonValue `json:"value"`
}

type webhookPatchJsonValue struct {
	Key string `json:"key"`
	Operator string `json:"operator"`
	Values [] string `json:"values"`
}

type ReconcileAction struct {
}

func (a *ReconcileAction) Run(version, profile string, config []reconciler.Configuration, context *service.ActionContext) error {
	var overrides = make(map[string]interface{}, len(config))
	for _, configEntry := range config {
		overrides[configEntry.Key] = configEntry.Value
	}
	component := chart.NewComponentBuilder(version, istioChart).WithNamespace(istioNamespace).WithProfile(profile).WithConfiguration(config).Build()
	manifest, err := context.ChartProvider.RenderManifest(component)
	if err != nil {
		return err
	}

	istioOperator := extractIstioOperatorContextFrom(manifest.Manifest)
	istioOperatorPath, istioOperatorCf, err := file.CreateTempFileWith(istioOperator)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := istioOperatorCf()
		if cleanupErr != nil {
			context.Logger.Error(cleanupErr)
		}
	}()

	kubeconfigPath, kubeconfigCf, err := file.CreateTempFileWith(context.KubeClient.Kubeconfig())
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			context.Logger.Error(cleanupErr)
		}
	}()

	istioBinaryPath := getIstioctlBinaryPath()
	cmd := prepareIstioctlCommand(istioBinaryPath, istioOperatorPath, kubeconfigPath)
	if err := cmd.Run(); err != nil {
		return err
	}

	_, err = context.KubeClient.Deploy(context.Context, manifest.Manifest, istioNamespace)
	if err != nil {
		return err
	}

	patchContent := []webhookPatchJson{{
		Op:    "add",
		Path:  "/webhooks/4/namespaceSelector/matchExpressions/-",
		Value: webhookPatchJsonValue{
			Key:      "gardener.cloud/purpose",
			Operator: "NotIn",
			Values:   []string{
				"kube-system",
			},
		},
	}}

	patchContentJson, err := json.Marshal(patchContent)
	if err != nil {
		return err
	}

	err = context.KubeClient.PatchUsingStrategy("MutatingWebhookConfiguration", "istio-sidecar-injector", istioNamespace, patchContentJson, types.JSONPatchType)
	if err != nil {
		return err
	}

	return nil
}

func getIstioctlBinaryPath() string {
	return os.Getenv(istioctlBinaryPathEnvKey)
}

func prepareIstioctlCommand(istioBinaryPath, istioOperatorPath, kubeconfigPath string) *exec.Cmd {
	cmd := exec.Command(istioBinaryPath, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

func extractIstioOperatorContextFrom(manifest string) string {
	defs := strings.Split(manifest, yamlDelimiter)
	for _, def := range defs {
		if strings.Contains(def, istioOperatorKind) {
			return def
		}
	}

	return ""
}
