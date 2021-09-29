package istio

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"strings"
)

const (
	istioNamespace = "istio-system"
	istioChart     = "istio-configuration"
)

type ReconcileAction struct {
}

func (a *ReconcileAction) Run(context *service.ActionContext) error {
	component := chart.NewComponentBuilder(context.Model.Version, istioChart).
		WithNamespace(istioNamespace).
		WithProfile(context.Model.Profile).
		WithConfiguration(context.Model.Configuration).Build()
	manifest, err := context.ChartProvider.RenderManifest(component)
	if err != nil {
		return err
	}

	commander := &istioctl.DefaultCommander{
		Logger: context.Logger,
	}
	performer, err := actions.NewDefaultIstioPerformer(context.KubeClient.Kubeconfig(), manifest.Manifest, context.KubeClient, context.Logger, commander)
	if err != nil {
		return errors.Wrap(err, "Could not initialize DefaultIstioPerformer")
	}

	err = performer.Install()
	if err != nil {
		return errors.Wrap(err, "Could not install Istio")
	}

	err = performer.PatchMutatingWebhook()
	if err != nil {
		return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
	}

	generated := generateNewManifestWithoutIstioOperatorFrom(manifest.Manifest)
	fmt.Println(generated)
	_, err = context.KubeClient.Deploy(context.Context, generated, istioNamespace, nil)
	if err != nil {
		return errors.Wrap(err, "Could not deploy Istio resources")
	}

	return nil
}

func generateNewManifestWithoutIstioOperatorFrom(manifest string) string {
	unstructs, err := kubeclient.ToUnstructured([]byte(manifest), true)
	if err != nil {
		return ""
	}

	builder := strings.Builder{}
	for _, unstruct := range unstructs {
		if unstruct.GetKind() == "IstioOperator" {
			continue
		}

		unstructBytes, err := unstruct.MarshalJSON()
		if err != nil {
			return ""
		}

		builder.WriteString("---\n")
		builder.WriteString(string(unstructBytes))
	}

	return builder.String()
}
