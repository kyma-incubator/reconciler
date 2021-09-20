package istio

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"strings"
	"helm.sh/helm/v3/pkg/chart/loader"
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
		appVersion, err := getAppVersionFromChart(context, version)
		if err != nil {
			return err
		}

		if !isClientVersionAcceptable(ver, appVersion) {
			return errors.Errorf("Istio could not be updated since the binary version: %s is not up to date: %s", ver.ClientVersion, appVersion)
		}

		if isDowngrade(ver, appVersion) {
			return errors.Errorf("Istio does not need to be downgraded from version: %s to version: %s", ver.PilotVersion, appVersion)
		}

		if !canUpdate(ver, appVersion) {
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

// Gets the appVersion to upgrade to from Chart.yml using the helm client
func getAppVersionFromChart(context *service.ActionContext, version string) (string, error) {
	ws, err := context.WorkspaceFactory.Get(version)
	if err != nil {
		return "", err
	}
	chart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return "", err
	}
	return chart.Metadata.AppVersion, nil
}

// shouldInstall checks if istio is already installed
func shouldInstall(version actions.IstioVersion) bool {
	return version.DataPlaneVersion == "" && version.PilotVersion == ""
}

// isClientVersionAcceptable checks if istioctl version is up to date.
func isClientVersionAcceptable(version actions.IstioVersion, appVersion string) bool {
	return version.ClientVersion == appVersion
}

// canUpdate checks if the update required is different by one minor version.
func canUpdate(version actions.IstioVersion, appVersion string) bool {
	pilotVersionSlice := strings.Split(version.ClientVersion, ".")
	appVersionSlice := strings.Split(appVersion, ".")

	pilotMinorVersion, err := strconv.Atoi(pilotVersionSlice[1])
	if err != nil {
		return false
	}
	appMinorVersion, err := strconv.Atoi(appVersionSlice[1])
	if err != nil {
		return false
	}
	if appVersionSlice[0] == pilotVersionSlice[0] && appMinorVersion-pilotMinorVersion > 1 {
		return false
	}
	return true
}

// isMismatchPresent checks if there is mismatch between Pilot and DataPlane versions.
func isMismatchPresent(version actions.IstioVersion) bool {
	return version.PilotVersion != version.DataPlaneVersion
}

// isDowngrade checks if we are downgrading Istio.
func isDowngrade(version actions.IstioVersion, appVersion string) bool {
	pilotVersionSlice := strings.Split(version.ClientVersion, ".")
	appVersionSlice := strings.Split(appVersion, ".")

	pilotMinorVersion, err := strconv.Atoi(pilotVersionSlice[1])
	if err != nil {
		return false
	}
	appMinorVersion, err := strconv.Atoi(appVersionSlice[1])
	if err != nil {
		return false
	}
	pilotSubMinorVersion, err := strconv.Atoi(pilotVersionSlice[2])
	if err != nil {
		return false
	}
	appSubMinorVersion, err := strconv.Atoi(appVersionSlice[2])
	if err != nil {
		return false
	}

	if appVersionSlice[0] == pilotVersionSlice[0] {
		if appMinorVersion > pilotMinorVersion {
			return false
		} else if appMinorVersion == pilotMinorVersion {
			if appSubMinorVersion >= pilotSubMinorVersion {
				return false
			}
		}
	}
	return true
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
