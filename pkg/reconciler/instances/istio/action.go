package istio

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/coreos/go-semver/semver"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/label"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
)

const (
	istioNamespace = "istio-system"
)

type bootstrapIstioPerformer func(logger *zap.SugaredLogger) (actions.IstioPerformer, error)

type StatusPreAction struct {
	getIstioPerformer bootstrapIstioPerformer
}

func NewStatusPreAction(getIstioPerformer bootstrapIstioPerformer) *StatusPreAction {
	return &StatusPreAction{getIstioPerformer}
}

func (a *StatusPreAction) Run(context *service.ActionContext) error {
	context.Logger.Debug("Pre reconcile action of istio triggered")

	performer, err := a.getIstioPerformer(context.Logger)
	if err != nil {
		return err
	}

	istioStatus, err := getInstalledVersion(context, performer)
	if err != nil {
		return err
	}

	if !isClientCompatibleWithTargetVersion(istioStatus) {
		return fmt.Errorf("Istio could not be updated since the binary version: %s is not compatible with the target version: %s - the difference between versions exceeds one minor version", istioStatus.ClientVersion, istioStatus.TargetVersion)
	}
	context.Logger.Debug("Pre version check successful")

	return nil
}

type MainReconcileAction struct {
	getIstioPerformer bootstrapIstioPerformer
}

func NewIstioMainReconcileAction(getIstioPerformer bootstrapIstioPerformer) *MainReconcileAction {
	return &MainReconcileAction{getIstioPerformer}
}

func (a *MainReconcileAction) Run(context *service.ActionContext) error {
	context.Logger.Debug("Reconcile action of istio triggered")

	performer, err := a.getIstioPerformer(context.Logger)
	if err != nil {
		return err
	}

	err = deployIstio(context, performer)
	errPatchMutatingWebhook := performer.PatchMutatingWebhook(context.Context, context.KubeClient,
		context.WorkspaceFactory, context.Task.Version, context.Task.Component, context.Logger)
	if errPatchMutatingWebhook != nil {
		errPatchMutatingWebhook = errors.Wrap(errPatchMutatingWebhook, "Could not patch MutatingWebhookConfiguration")
		if err != nil {
			err = errors.Wrap(err, errPatchMutatingWebhook.Error())
		} else {
			err = errPatchMutatingWebhook
		}
	}

	errLabelNamespaces := performer.LabelNamespaces(context.Context, context.KubeClient,
		context.WorkspaceFactory, context.Task.Version, context.Task.Component, context.Logger)
	if errLabelNamespaces != nil {
		errLabelNamespaces = errors.Wrap(errLabelNamespaces, "Could not label namespaces")
		if err != nil {
			err = errors.Wrap(err, errLabelNamespaces.Error())
		} else {
			err = errLabelNamespaces
		}
	}

	return err
}

func deployIstio(context *service.ActionContext, performer actions.IstioPerformer) error {
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

	if canInstall(istioStatus) {
		context.Logger.Info("No Istio version was detected on the cluster, performing installation...")

		err = performer.Install(context.KubeClient.Kubeconfig(), istioManifest.Manifest, istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not install Istio")
		}

	} else if canUpdateResult, err := canUpdate(istioStatus); canUpdateResult {
		context.Logger.Debugf("Istio version was detected on the cluster, updating pilot from %s and data plane from %s to version %s...", istioStatus.PilotVersion, istioStatus.DataPlaneVersion, istioStatus.TargetVersion)

		err = performer.Update(context.KubeClient.Kubeconfig(), istioManifest.Manifest, istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not update Istio")
		}
	} else {
		return err
	}

	return nil
}

type ProxyResetPostAction struct {
	getIstioPerformer bootstrapIstioPerformer
}

func NewProxyResetPostAction(getIstioPerformer bootstrapIstioPerformer) *ProxyResetPostAction {
	return &ProxyResetPostAction{getIstioPerformer}
}

func (a *ProxyResetPostAction) Run(context *service.ActionContext) error {
	context.Logger.Debug("Proxy reset post action of istio triggered")

	performer, err := a.getIstioPerformer(context.Logger)
	if err != nil {
		return err
	}

	istioStatus, err := getInstalledVersion(context, performer)
	if err != nil {
		return err
	}

	canUpdateResult, err := canUpdate(istioStatus)
	if err != nil {
		context.Logger.Warnf("could not perform ResetProxy action: %v", err)
		return nil
	}

	err = performer.ResetProxy(context.Context, context.KubeClient.Kubeconfig(), context.WorkspaceFactory, context.Task.Version, context.Task.Component, istioStatus.TargetVersion, istioStatus.TargetPrefix, context.Logger, canUpdateResult)
	if err != nil {
		context.Logger.Warnf("could not perform ResetProxy action: %v", err)
		return nil
	}

	return nil
}

type LabelWarningsPostAction struct {
	action label.Action
}

func NewLabelWarningsPostAction(action label.Action) *LabelWarningsPostAction {
	return &LabelWarningsPostAction{action: action}
}

func (a *LabelWarningsPostAction) Run(context *service.ActionContext) error {
	const (
		retriesCount        = 5
		delayBetweenRetries = 5 * time.Second
		timeout             = 5 * time.Minute
		interval            = 12 * time.Second
	)

	context.Logger.Debug("Label pods out Istio mesh action triggered")

	logger := context.Logger.With("istio-label")

	sidecarInjectionEnabledByDefault, err := actions.IsSidecarInjectionNamespacesByDefaultEnabled(context.WorkspaceFactory, context.Task.Version, context.Task.Component)
	if err != nil {
		logger.Error("Could not retrieve default istio sidecar injection!")
		return err
	}

	client, err := context.KubeClient.Clientset()
	if err != nil {
		return err
	}

	retryOpts := []retry.Option{
		retry.Delay(delayBetweenRetries),
		retry.Attempts(uint(retriesCount)),
		retry.DelayType(retry.FixedDelay),
	}

	err = a.action.Run(context.Context, logger, client, retryOpts, sidecarInjectionEnabledByDefault)
	if err != nil {
		return err
	}

	return nil
}

type UninstallAction struct {
	getIstioPerformer bootstrapIstioPerformer
}

// NewUninstallAction returns an instance of UninstallAction
func NewUninstallAction(getIstioPerformer bootstrapIstioPerformer) *UninstallAction {
	return &UninstallAction{getIstioPerformer}
}

func (a *UninstallAction) Run(context *service.ActionContext) error {
	context.Logger.Debug("Uninstall action of istio triggered")

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
		// Before removing istio himself, undeploy all related objects like dashboards
		err = unDeployIstioRelatedResources(context.Context, istioManifest.Manifest, context.KubeClient, context.Logger)
		if err != nil {
			return err
		}
		err = performer.Uninstall(context.KubeClient, istioStatus.TargetVersion, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not uninstall istio")
		}
		context.Logger.Debugf("Istio successfully uninstalled")
	} else {
		context.Logger.Warnf("Istio is not installed, can not uninstall it")
	}
	return nil
}

type helperVersion struct {
	ver semver.Version
}

func (h helperVersion) compare(second helperVersion) int {
	if h.ver.Major > second.ver.Major {
		return 1
	} else if h.ver.Major == second.ver.Major {
		if h.ver.Minor > second.ver.Minor {
			return 1
		} else if h.ver.Minor == second.ver.Minor {
			if h.ver.Patch > second.ver.Patch {
				return 1
			} else if h.ver.Patch == second.ver.Patch {
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

func newHelperVersionFrom(versionInString string) (helperVersion, error) {
	version, err := semver.NewVersion(versionInString)
	if err != nil {
		return helperVersion{}, err
	}
	return helperVersion{ver: *version}, err
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
	context.Logger.Debugf("Detected: istioctl version %s, target Istio version: %s", istioStatus.ClientVersion, istioStatus.TargetVersion)
	return istioStatus, nil
}

func isClientCompatibleWithTargetVersion(istioStatus actions.IstioStatus) bool {

	clientHelperVersion, err := newHelperVersionFrom(istioStatus.ClientVersion)
	if err != nil {
		return false
	}
	targetHelperVersion, err := newHelperVersionFrom(istioStatus.TargetVersion)
	if err != nil {
		return false
	}

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
	if componentVersion == "" {
		return true, nil
	}
	componentHelperVersion, err := newHelperVersionFrom(componentVersion)
	if err != nil {
		return false, err
	}
	targetHelperVersion, err := newHelperVersionFrom(targetVersion)
	if err != nil {
		return false, err
	}

	componentVsTargetComparison := targetHelperVersion.compare(componentHelperVersion)
	if !amongOneMinor(componentHelperVersion, targetHelperVersion) {
		return false, fmt.Errorf("Could not perform %s for %s from version: %s to version: %s - the difference between versions exceed one minor version",
			getActionTypeFrom(componentVsTargetComparison), componentName, componentVersion, targetVersion)
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
	return first.ver.Major == second.ver.Major && (first.ver.Minor == second.ver.Minor || first.ver.Minor-second.ver.Minor == -1 || first.ver.Minor-second.ver.Minor == 1)
}

func unDeployIstioRelatedResources(context context.Context, manifest string, client kubernetes.Client, logger *zap.SugaredLogger) error {
	logger.Debugf("Undeploying istio related dashboards")
	// multiple calls necessary, please see: https://github.com/kyma-incubator/reconciler/issues/367
	_, err := client.Delete(context, manifest, "kyma-system")
	if err != nil {
		return err
	}
	logger.Debugf("Undeploying other istio related resources")
	_, err = client.Delete(context, manifest, istioNamespace)
	if err != nil {
		return err
	}

	return nil
}
