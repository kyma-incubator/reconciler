package istio

import (
	"bytes"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	istioctl1_11_1 = "/bin/istioctl-1.11.1" // change path to the local `istioctl` if debugging locally
	yaml_delimiter = "---"
	istio_operator_kind = "kind: IstioOperator"
)

type ReconcileAction struct {
}

func (a *ReconcileAction) Run(version, profile string, config []reconciler.Configuration, context *service.ActionContext) error {
	var overrides = make(map[string]interface{}, len(config))

	for _, configEntry := range config {
		overrides[configEntry.Key] = configEntry.Value
	}

	comp := chart.NewComponent("istio-configuration", "istio-system", overrides)

	componentSet := chart.NewComponentSet(context.KubeClient.Kubeconfig(), version, profile, []*chart.Component{comp})
	log.Println("Before rendering the manifest")

	manifests, err := context.ChartProvider.Manifests(componentSet, false, &chart.Options{})
	if err != nil {
		return err
	}

	manifestsCount := len(manifests)
	if manifestsCount != 1  {
		return errors.Errorf("One manifest expected but got %s", manifestsCount)
	}
	finalManifest := manifests[0].Manifest

	istioOperator := extractIstioOperatorContextFrom(finalManifest)
	istioOperatorPath, istioOperatorCf, err := file.Path(istioOperator)
	if err != nil {
		return err
	}

	context.Logger.Infof(istioOperatorPath)

	defer func() {
		cleanupErr := istioOperatorCf()
		if cleanupErr != nil {
			context.Logger.Error(cleanupErr)
		}
	}()

	kubeconfiigPath, kubeconfigCf, err := file.Path(context.KubeClient.Kubeconfig())
	if err != nil {
		return err
	}

	context.Logger.Infof(kubeconfiigPath)

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			context.Logger.Error(cleanupErr)
		}
	}()


	cmd := prepareIstioctlCommand(istioOperatorPath, kubeconfiigPath)
	if err := cmd.Run(); err != nil {
		return err
	}

	_, err = context.KubeClient.Deploy(nil, finalManifest, "istio-system", nil)
	if err != nil {
		return err
	}

	// TODO: add patching to the Istio

	return nil
}

func prepareIstioctlCommand(istioOperatorPath, kubeconfiigPath string) *exec.Cmd {
	cmd := exec.Command(istioctl1_11_1, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfiigPath, "--skip-confirmation")
	var stdBuffer bytes.Buffer
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw

	return cmd
}

func extractIstioOperatorContextFrom(manifest string) string {
	defs := strings.Split(manifest, yaml_delimiter)
	for _, def := range defs {
		if strings.Contains(def, istio_operator_kind) {
			fmt.Println(def)
			return def
		}
	}

	return ""
}
