package istio

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/manifest"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
)

const (
	istioNamespace = "istio-system"
)

type bootstrapIstioPerformer func(logger *zap.SugaredLogger) (actions.IstioPerformer, error)

type ReconcileAction struct {
	//Temporary solution to overcome Reconciler limitation: Unable to bootstrap IstioPerformer only once in the component reconciler lifetime
	getIstioPerformer bootstrapIstioPerformer
}

// NewReconcileAction returns an instance of ReconcileAction
func NewReconcileAction(getIstioPerformer bootstrapIstioPerformer) *ReconcileAction {
	return &ReconcileAction{getIstioPerformer}
}

type UninstallAction struct {
	getIstioPerformer bootstrapIstioPerformer
}

// NewUninstallAction returns an instance of UninstallAction
func NewUninstallAction(getIstioPerformer bootstrapIstioPerformer) *UninstallAction {
	return &UninstallAction{getIstioPerformer}
}

func (a *UninstallAction) Run(context *service.ActionContext) error {
	context.Logger.Debugf("Uninstall action of istio triggered")

	performer, err := a.getIstioPerformer(context.Logger)
	if err != nil {
		return err
	}

	istioStatus, err := getInstalledVersion(context, performer)
	if err != nil {
		return err
	}
	if canUninstall(istioStatus) {
		component := chart.NewComponentBuilder(context.Task.Version, context.Task.Component).
			WithNamespace(istioNamespace).
			WithProfile(context.Task.Profile).
			WithConfiguration(context.Task.Configuration).Build()
		istioManifest, err := context.ChartProvider.RenderManifest(component)
		if err != nil {
			return err
		}
		//Before removing istio himself, undeploy all related objects like dashboards
		err = unDeployIstioRelatedResources(context.Context, istioManifest.Manifest, context.KubeClient, context.Logger)
		if err != nil {
			return err
		}
		err = performer.Uninstall(context.KubeClient, istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not uninstall istio")
		}
		context.Logger.Infof("Istio successfully uninstalled")
	} else {
		context.Logger.Warnf("Istio is not installed, can not uninstall it")
	}
	return nil
}

func (a *ReconcileAction) Run(context *service.ActionContext) error {
	context.Logger.Debugf("Reconcile action of istio triggered")

	performer, err := a.getIstioPerformer(context.Logger)
	if err != nil {
		return err
	}

	component := chart.NewComponentBuilder(context.Task.Version, context.Task.Component).
		WithNamespace(istioNamespace).
		WithProfile(context.Task.Profile).
		WithConfiguration(context.Task.Configuration).Build()
	istioManifest, err := context.ChartProvider.RenderManifest(component)
	if err != nil {
		return err
	}

	istioStatus, err := getInstalledVersion(context, performer)
	if err != nil {
		return err
	}

	if isMismatchPresent(istioStatus) {
		errorMessage := fmt.Sprintf("Istio components version mismatch detected: pilot version: %s, data plane version: %s", istioStatus.PilotVersion, istioStatus.DataPlaneVersion)
		return errors.New(errorMessage)
	}

	if !isClientCompatibleWithTargetVersion(istioStatus) {
		return errors.New(fmt.Sprintf("Istio could not be updated since the binary version: %s is not compatible with the target version: %s - the difference between versions exceeds one minor version", istioStatus.ClientVersion, istioStatus.TargetVersion))
	}

	if canInstall(istioStatus) {
		context.Logger.Info("No Istio version was detected on the cluster, performing installation...")

		err = performer.Install(context.KubeClient.Kubeconfig(), istioManifest.Manifest, istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not install Istio")
		}

		err = performer.PatchMutatingWebhook(context.Context, context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
		}
	} else if canUpdateResult, err := canUpdate(istioStatus); canUpdateResult {
		context.Logger.Infof("Istio version was detected on the cluster, updating pilot from %s and data plane from %s to version %s...", istioStatus.PilotVersion, istioStatus.DataPlaneVersion, istioStatus.TargetVersion)

		err = performer.Update(context.KubeClient.Kubeconfig(), istioManifest.Manifest, istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not update Istio")
		}

		err = performer.PatchMutatingWebhook(context.Context, context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
		}

		err = performer.ResetProxy(context.Context, context.KubeClient.Kubeconfig(), istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not reset Istio proxy")
		}
	} else {
		return err
	}

	return nil
}

type ReconcileIstioConfigurationAction struct {
	getIstioPerformer bootstrapIstioPerformer
}

// NewReconcileIstioConfigurationAction returns an instance of ReconcileIstioConfigurationAction
func NewReconcileIstioConfigurationAction(getIstioPerformer bootstrapIstioPerformer) *ReconcileAction {
	return (*ReconcileAction)(&ReconcileIstioConfigurationAction{getIstioPerformer})
}

func (a *ReconcileIstioConfigurationAction) Run(context *service.ActionContext) error {
	context.Logger.Debugf("Reconcile action of istio-configuration triggered")

	performer, err := a.getIstioPerformer(context.Logger)
	if err != nil {
		return err
	}

	component := chart.NewComponentBuilder(context.Task.Version, context.Task.Component).
		WithNamespace(istioNamespace).
		WithProfile(context.Task.Profile).
		WithConfiguration(context.Task.Configuration).Build()
	istioManifest, err := context.ChartProvider.RenderManifest(component)
	if err != nil {
		return err
	}

	istioStatus, err := getInstalledVersion(context, performer)
	if err != nil {
		return err
	}

	if isMismatchPresent(istioStatus) {
		errorMessage := fmt.Sprintf("Istio components version mismatch detected: pilot version: %s, data plane version: %s", istioStatus.PilotVersion, istioStatus.DataPlaneVersion)
		return errors.New(errorMessage)
	}

	if !isClientCompatibleWithTargetVersion(istioStatus) {
		return errors.New(fmt.Sprintf("Istio could not be updated since the binary version: %s is not compatible with the target version: %s - the difference between versions exceeds one minor version", istioStatus.ClientVersion, istioStatus.TargetVersion))
	}

	if canInstall(istioStatus) {
		context.Logger.Info("No Istio version was detected on the cluster, performing installation...")

		err = performer.Install(context.KubeClient.Kubeconfig(), istioManifest.Manifest, istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not install Istio")
		}

		err = performer.PatchMutatingWebhook(context.Context, context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
		}

		err = deployIstioResources(context.Context, istioManifest.Manifest, context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not deploy Istio resources")
		}
	} else if canUpdateResult, err := canUpdate(istioStatus); canUpdateResult {
		context.Logger.Infof("Istio version was detected on the cluster, updating pilot from %s and data plane from %s to version %s...", istioStatus.PilotVersion, istioStatus.DataPlaneVersion, istioStatus.TargetVersion)

		err = performer.Update(context.KubeClient.Kubeconfig(), istioManifest.Manifest, istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not update Istio")
		}

		err = performer.PatchMutatingWebhook(context.Context, context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
		}

		err = performer.ResetProxy(context.Context, context.KubeClient.Kubeconfig(), istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not reset Istio proxy")
		}

		err = deployIstioResources(context.Context, istioManifest.Manifest, context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not deploy Istio resources")
		}
	} else {
		return err
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

func canInstall(istioStatus actions.IstioStatus) bool {
	return !isInstalled(istioStatus)
}

func isInstalled(istioStatus actions.IstioStatus) bool {
	return !(istioStatus.DataPlaneVersion == "" && istioStatus.PilotVersion == "")
}

func canUninstall(istioStatus actions.IstioStatus) bool {
	return isInstalled(istioStatus) && istioStatus.ClientVersion != ""
}

func getInstalledVersion(context *service.ActionContext, performer actions.IstioPerformer) (actions.IstioStatus, error) {
	istioStatus, err := performer.Version(context.WorkspaceFactory, context.Task.Version, context.Task.Component, context.KubeClient.Kubeconfig(), context.Logger)
	if err != nil {
		return actions.IstioStatus{}, errors.Wrap(err, "Could not fetch Istio version")
	}
	context.Logger.Infof("Detected: istioctl version %s, target Istio version: %s", istioStatus.ClientVersion, istioStatus.TargetVersion)
	return istioStatus, nil
}

func isClientCompatibleWithTargetVersion(istioStatus actions.IstioStatus) bool {

	clientHelperVersion := newHelperVersionFrom(istioStatus.ClientVersion)
	targetHelperVersion := newHelperVersionFrom(istioStatus.TargetVersion)

	return amongOneMinor(clientHelperVersion, targetHelperVersion)
}

func canUpdate(istioStatus actions.IstioStatus) (bool, error) {
	if isPilotCompatible, err := isComponentCompatible(istioStatus.PilotVersion, istioStatus.TargetVersion, "Pilot"); !isPilotCompatible {
		return false, err
	}

	if isDataplaneCompatible, err := isComponentCompatible(istioStatus.DataPlaneVersion, istioStatus.TargetVersion, "Data plane"); !isDataplaneCompatible {
		return false, err
	}

	return true, nil
}

func isComponentCompatible(componentVersion, targetVersion, componentName string) (bool, error) {
	componentHelperVersion := newHelperVersionFrom(componentVersion)
	targetHelperVersion := newHelperVersionFrom(targetVersion)

	componentVsTargetComparison := targetHelperVersion.compare(&componentHelperVersion)
	if !amongOneMinor(componentHelperVersion, targetHelperVersion) {
		return false, errors.New(fmt.Sprintf("Could not perform %s for %s from version: %s to version: %s - the difference between versions exceed one minor version",
			getActionTypeFrom(componentVsTargetComparison), componentName, componentVersion, targetVersion))
	}

	return true, nil
}

func getActionTypeFrom(comparison int) string {
	switch comparison {
	case 1:
		return "upgrade"
	case 0:
		return "reconciliation"
	case -1:
		return "downgrade"
	default:
		return "unknown"
	}
}

func amongOneMinor(first, second helperVersion) bool {
	return first.major == second.major && (first.minor == second.minor || first.minor-second.minor == -1 || first.minor-second.minor == 1)
}

func isMismatchPresent(istioStatus actions.IstioStatus) bool {
	pilot := newHelperVersionFrom(istioStatus.PilotVersion)
	dataPlane := newHelperVersionFrom(istioStatus.DataPlaneVersion)
	return pilot.compare(&dataPlane) != 0
}

func deployIstioResources(context context.Context, chartManifest string, client kubernetes.Client, logger *zap.SugaredLogger) error {
	generated, err := manifest.GenerateNewManifestWithoutIstioOperatorFrom(chartManifest)
	if err != nil {
		return errors.Wrap(err, "Could not generate manifest without Istio Operator")
	}

	logger.Infof("Deploying other Istio resources...")
	_, err = client.Deploy(context, generated, istioNamespace, nil)
	if err != nil {
		return err
	}

	return nil
}

func unDeployIstioRelatedResources(context context.Context, manifest string, client kubernetes.Client, logger *zap.SugaredLogger) error {
	logger.Infof("Undeploying istio related dashboards")
	//multiple calls necessary, please see: https://github.com/kyma-incubator/reconciler/issues/367
	_, err := client.Delete(context, manifest, "kyma-system")
	if err != nil {
		return err
	}
	logger.Infof("Undeploying other istio related resources")
	_, err = client.Delete(context, manifest, istioNamespace)
	if err != nil {
		return err
	}

	return nil
}
