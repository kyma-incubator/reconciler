package istio

import (
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
	performer actions.IstioPerformer
	commander istioctl.Commander
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

	ver, err := a.performer.Version(context.KubeClient.Kubeconfig(), context.Logger, a.commander)
	if err != nil {
		return errors.Wrap(err, "Could not fetch Istio version")
	}

	context.Logger.Infof("Detected versions: istioctl %s, pilot version: %s, data plane version: %s", ver.ClientVersion, ver.PilotVersion, ver.DataPlaneVersion)

	if shouldInstall(ver) {
		commander := &istioctl.DefaultCommander{
		Logger: context.Logger,
	}
	performer, err := actions.NewDefaultIstioPerformer(context.KubeClient.Kubeconfig(), manifest.Manifest, context.KubeClient, context.Logger, commander)
	if err != nil {
		return errors.Wrap(err, "Could not initialize DefaultIstioPerformer")
	}

	err = performer.Install(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger, a.commander)err = a.performer.Install(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger, a.commander)
		context.Logger.Info("No Istio version was detected on the cluster, performing installation...")

		err = a.performer.Install(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger, a.commander)
		if err != nil {
			return errors.Wrap(err, "Could not install Istio")
		}

		err = a.performer.PatchMutatingWebhook(context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
		}
	} else {
		if !canUpdate(ver) {
			return errors.New("Istio could not be updated due to the versions limitations")
		}

		if !isMismatchPresent(ver) {
			context.Logger.Warnf("Istio components version mismatch detected: pilot version: %s, data plane version: %s", ver.PilotVersion, ver.DataPlaneVersion)
		}

		context.Logger.Infof("Istio version was detected on the cluster, updating from %s to %s...", ver.PilotVersion, ver.ClientVersion)

		err = a.performer.Update(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger, a.commander)
		if err != nil {
			return errors.Wrap(err, "Could not update Istio")
		}
	}

	generated, err := generateNewManifestWithoutIstioOperatorFrom(manifest.Manifest)
	if err != nil {
		return errors.Wrap(err, "Could not generate manifest without Istio Operator")
	}

	_, err = context.KubeClient.Deploy(context.Context, generated, istioNamespace, nil)
	if err != nil {
		return errors.Wrap(err, "Could not deploy Istio resources")
	}

	return nil
}

func shouldInstall(version actions.IstioVersion) bool {
	if version.DataPlaneVersion == "" && version.PilotVersion == "" {
		return true
	} else {
		return false
	}
}

func canUpdate(version actions.IstioVersion) bool {
	// mockup function
	return false
}

func isMismatchPresent(version actions.IstioVersion) bool {
	if version.PilotVersion != version.DataPlaneVersion {
		return true
	} else {
		return false
	}
}

func generateNewManifestWithoutIstioOperatorFrom(manifest string) (string, error) {
	unstructs, err := kubeclient.ToUnstructured([]byte(manifest), true)
	if err != nil {
		return "", err
	}

	builder := strings.Builder{}
	for _, unstruct := range unstructs {
		if unstruct.GetKind() == "IstioOperator" {
			continue
		}

		unstructBytes, err := unstruct.MarshalJSON()
		if err != nil {
			return "", err
		}

		builder.WriteString("---\n")
		builder.WriteString(string(unstructBytes))
	}

	return builder.String(), nil
}
