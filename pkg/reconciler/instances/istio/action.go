package istio

import (
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
)

const (
	istioNamespace = "istio-system"
	istioChart     = "istio-configuration"
)

type ReconcileAction struct {
	performer actions.IstioPerformer
}

type IstioOperation struct {
}

func (io *IstioOperation) shouldInstall() (bool, error) {
	return false, nil
}
func (io *IstioOperation) shouldReinstall() (bool, error) {
	return false, nil
}
func (io *IstioOperation) shouldUpdate() (bool, error) {
	return false, nil
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

	ver, err := a.performer.Version(context.WorkspaceFactory, context.Model.Version, istioChart, context.KubeClient.Kubeconfig(), context.Logger)
	if err != nil {
		return errors.Wrap(err, "Could not fetch Istio version")
	}

	context.Logger.Infof("Detected: istioctl version %s, target Istio version: %s", ver.ClientVersion, ver.TargetVersion)

	if isMismatchPresent(ver) {
		context.Logger.Warnf("Istio components version mismatch detected: pilot version: %s, data plane version: %s", ver.PilotVersion, ver.DataPlaneVersion)
	}

	istioOperatorManifest, err := extractIstioOperatorContextFrom(manifest.Manifest)
	if err != nil {
		return errors.Wrap(err, "Could not generate Istio Operator manifest")
	}

	if canInstall(ver) {
		context.Logger.Info("No Istio version was detected on the cluster, performing installation...")

		err = a.performer.Install(context.KubeClient.Kubeconfig(), istioOperatorManifest, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not install Istio")
		}

		err = a.performer.PatchMutatingWebhook(context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
		}

		generated, err := generateNewManifestWithoutIstioOperatorFrom(manifest.Manifest)
		if err != nil {
			return errors.Wrap(err, "Could not generate manifest without Istio Operator")
		}

		_, err = context.KubeClient.Deploy(context.Context, generated, istioNamespace, nil)
		if err != nil {
			return errors.Wrap(err, "Could not deploy Istio resources")
		}
	} else if canUpdate(ver, context.Logger) {
		context.Logger.Infof("Istio version was detected on the cluster, updating pilot from %s and data plane from %s to version %s...", ver.PilotVersion, ver.DataPlaneVersion, ver.TargetVersion)

		err = a.performer.Update(context.KubeClient.Kubeconfig(), istioOperatorManifest, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not update Istio")
		}

		err = a.performer.ResetProxy(context.KubeClient.Kubeconfig(), ver, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not reset Istio proxy")
		}
	}

	return nil
}

type helperVersion struct {
	major int
	minor int
	patch int
}

func (h *helperVersion) compare(other *helperVersion) int {
	if h.major > other.major {
		return 1
	} else if h.major == other.major {
		if h.minor > other.minor {
			return 1
		} else if h.minor == other.minor {
			if h.patch > other.patch {
				return 1
			} else if h.patch == other.patch {
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

func canInstall(version actions.IstioVersion) bool {
	return version.DataPlaneVersion == "" && version.PilotVersion == ""
}

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

	if !maxOneMinorBehind(pilotHelperVersion, targetHelperVersion) || !maxOneMinorBehind(dataPlaneHelperVersion, targetHelperVersion) {
		logger.Errorf("Istio could not be updated from pilot: %s and data plane: %s to version: %s - versions different exceed one minor version",
			ver.PilotVersion, ver.DataPlaneVersion, ver.TargetVersion)
		return false
	}

	return true
}

func maxOneMinorBehind(client, target helperVersion) bool {
	return client.major == target.major && (target.minor-client.minor) <= 1
}

func isMismatchPresent(ver actions.IstioVersion) bool {
	pilot := newHelperVersionFrom(ver.PilotVersion)
	dataPlane := newHelperVersionFrom(ver.DataPlaneVersion)
	return pilot.compare(&dataPlane) != 0
}
