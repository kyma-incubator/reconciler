package istio

import (
	"go.uber.org/zap"
	"strconv"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart/loader"
)

const (
	istioNamespace        = "istio-system"
	istioChart            = "istio-configuration"
)

type ReconcileAction struct {
	performer       actions.IstioPerformer
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

	ver, err := a.performer.Version(context.WorkspaceFactory, version, istioChart, context.KubeClient.Kubeconfig(), context.Logger)
	if err != nil {
		return errors.Wrap(err, "Could not fetch Istio version")
	}

	context.Logger.Infof("Detected versions: istioctl %s, pilot version: %s, data plane version: %s", ver.ClientVersion, ver.PilotVersion, ver.DataPlaneVersion)

	if isMismatchPresent(ver) {
		context.Logger.Warnf("Istio components version mismatch detected: pilot version: %s, data plane version: %s", ver.PilotVersion, ver.DataPlaneVersion)
	}

	if canInstall(ver) {
		commander := &istioctl.DefaultCommander{
		Logger: context.Logger,
	}
	performer, err := actions.NewDefaultIstioPerformer(context.KubeClient.Kubeconfig(), manifest.Manifest, context.KubeClient, context.Logger, commander)
	if err != nil {
		return errors.Wrap(err, "Could not initialize DefaultIstioPerformer")
	}

	err = performer.Install(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger, a.commander)err = a.performer.Install(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger, a.commander)
		context.Logger.Info("No Istio version was detected on the cluster, performing installation...")

		err = a.performer.Install(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not install Istio")
		}

		err = a.performer.PatchMutatingWebhook(context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
		}
	} else if canUpdate(ver, context.Logger) {
		context.Logger.Infof("Istio version was detected on the cluster, updating from %s to %s...", ver.PilotVersion, ver.ClientVersion)

		err = a.performer.Update(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not update Istio")
		}

		err = a.performer.ResetProxy(context.KubeClient.Kubeconfig(), ver, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not reset Istio proxy")
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

type helperVersion struct {
	major int
	minor int
	patch int
}

func (h *helperVersion) compare(second *helperVersion) int {
	if h.major > second.major {
		return 1
	} else if h.major == second.major {
		if h.minor > second.minor {
			return 1
		} else if h.minor == second.minor {
			if h.patch > second.patch {
				return 1
			} else if h.patch == second.patch {
				return 0
			} else {
				return -1
			}
		} else {
			return -1
		}
	} else {
		return -1
	}
}

func newHelperVersionFrom(versionInString string) helperVersion {
	var major, minor, patch int

	versionsSliceByDot := strings.Split(versionInString, ".")
	valuesCount := len(versionsSliceByDot)

	if valuesCount > 2 {
		convertedPatchValue, err := strconv.Atoi(versionsSliceByDot[2])
		if err == nil {
			patch = convertedPatchValue
		}
	}

	if valuesCount > 1 {
		convertedMinorValue, err := strconv.Atoi(versionsSliceByDot[1])
		if err == nil {
			minor = convertedMinorValue
		}
	}

	if valuesCount > 0 {
		convertedMajorValue, err := strconv.Atoi(versionsSliceByDot[0])
		if err == nil {
			major = convertedMajorValue
		}
	}

	return helperVersion{
		major: major,
		minor: minor,
		patch: patch,
	}
}

// canUpgrade checks if it is possible to install Istio on the cluster.
func canInstall(version actions.IstioVersion) bool {
	return version.DataPlaneVersion == "" && version.PilotVersion == ""
}

// canUpgrade checks if it is possible to upgrade Istio on the cluster.
func canUpdate(ver actions.IstioVersion, logger *zap.SugaredLogger) bool {
	clientHelperVersion := newHelperVersionFrom(ver.ClientVersion)
	targetHelperVersion := newHelperVersionFrom(ver.TargetVersion)
	pilotHelperVersion := newHelperVersionFrom(ver.PilotVersion)
	dataPlaneHelperVersion := newHelperVersionFrom(ver.DataPlaneVersion)

	if !maxOneMinorBehind(clientHelperVersion, targetHelperVersion) {
		logger.Errorf("Istio could not be updated since the binary version: %s is not compatible with the target version: %s", ver.ClientVersion, ver.TargetVersion)
		return false
	}

	pilotVsTarget := targetHelperVersion.compare(&pilotHelperVersion)
	dataPlaneVsTarget := targetHelperVersion.compare(&dataPlaneHelperVersion)

	if pilotVsTarget == -1 || dataPlaneVsTarget == -1 {
		logger.Errorf("Downgrade detected from pilot: %s and data plane: %s to version: %s - finishing...", ver.PilotVersion, ver.DataPlaneVersion, ver.TargetVersion)
		return false
	}

	if pilotVsTarget == 0 && dataPlaneVsTarget == 0 {
		logger.Errorf("Current version: %s is the same as target version: %s - finishing...", ver.PilotVersion, ver.TargetVersion)
		return false
	}

	if !maxOneMinorBehind(pilotHelperVersion, targetHelperVersion) || !maxOneMinorBehind(dataPlaneHelperVersion, targetHelperVersion) {
		logger.Errorf("Istio could not be updated from pilot: %s and data plane: %s to version: %s - versions different exceed one minor version",
			ver.PilotVersion, ver.DataPlaneVersion, ver.TargetVersion)
		return false
	}

	return true
}

func maxOneMinorBehind(client, target helperVersion) bool {
	return client.major == target.major && target.minor-client.minor <= 1
}

// isMismatchPresent checks if there is mismatch between Pilot and DataPlane versions.
func isMismatchPresent(ver actions.IstioVersion) bool {
	pilot := newHelperVersionFrom(ver.PilotVersion)
	dataPlane := newHelperVersionFrom(ver.DataPlaneVersion)
	return pilot.compare(&dataPlane) != 0
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

func retrieveClientsetFrom(kubeConfig string, log *zap.SugaredLogger) (*kubernetes.Clientset, error) {
	kubeconfigPath, kubeconfigCf, err := file.CreateTempFileWith(kubeConfig)
	if err != nil {
		return nil, err
	}

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			log.Error(cleanupErr)
		}
	}()

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return kubeClient, nil
}
