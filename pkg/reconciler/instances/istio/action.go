package istio

import (
	"context"
	"fmt"
	"math"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/helpers"
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

	errPatchMutatingWebhook := performer.PatchMutatingWebhook(context.Context, context.KubeClient, context.Logger)
	if errPatchMutatingWebhook != nil {
		errPatchMutatingWebhook = errors.Wrap(errPatchMutatingWebhook, "Could not patch MutatingWebhookConfiguration")
	}

	switch {
	case err != nil && errPatchMutatingWebhook != nil:
		return errors.Wrap(err, errPatchMutatingWebhook.Error())

	case err != nil && errPatchMutatingWebhook == nil:
		return err

	case err == nil && errPatchMutatingWebhook != nil:
		return errPatchMutatingWebhook
	default:
		return nil
	}
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
	if canUpdateResult {
		err = performer.ResetProxy(context.Context, context.KubeClient.Kubeconfig(), istioStatus.TargetVersion, context.Logger)
		if err != nil {
			context.Logger.Warnf("could not perform ResetProxy action: %v", err)
			return nil
		}
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

func canInstall(istioStatus actions.IstioStatus) bool {
	return !isInstalled(istioStatus)
}

func isInstalled(istioStatus actions.IstioStatus) bool {
	return !(istioStatus.DataPlaneVersion == nil && istioStatus.PilotVersion == nil)
}

func canUninstall(istioStatus actions.IstioStatus) bool {
	return isInstalled(istioStatus) && istioStatus.ClientVersion.Library != ""
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
	return amongOneMinor(istioStatus.ClientVersion, istioStatus.TargetVersion)
}

func canUpdate(istioStatus actions.IstioStatus) (bool, error) {
	if isPilotCompatible, err := isComponentCompatible(*istioStatus.PilotVersion, istioStatus.TargetVersion, "Pilot"); !isPilotCompatible {
		return false, err
	}

	if isDataplaneCompatible, err := isComponentCompatible(*istioStatus.DataPlaneVersion, istioStatus.TargetVersion, "Data plane"); !isDataplaneCompatible {
		return false, err
	}

	return true, nil
}

func isComponentCompatible(componentVersion, targetVersion helpers.HelperVersion, componentName string) (bool, error) {

	componentVsTargetComparison := targetVersion.Compare(componentVersion)
	if !amongOneMinor(componentVersion, targetVersion) {
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

func amongOneMinor(first, second helpers.HelperVersion) bool {
	return first.Tag.Major == second.Tag.Major && int(math.Abs(float64(first.Tag.Minor)-float64(second.Tag.Minor))) < 2
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
