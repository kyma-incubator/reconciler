package istio

import (
	istioConfig "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"strconv"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
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
	istioImagePrefix      = "eu.gcr.io/kyma-project/external/istio/proxyv2"
	retriesCount          = 5
	delayBetweenRetries   = 10
	sleepAfterPodDeletion = 10
)

type ReconcileAction struct {
	commander       istioctl.Commander
	istioProxyReset proxy.IstioProxyReset
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

		err = a.performer.Install(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not install Istio")
		}

		err = a.performer.PatchMutatingWebhook(context.KubeClient, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not patch MutatingWebhookConfiguration")
		}
	} else {
		if !isClientVersionAcceptable(ver) {
			context.Logger.Errorf("Istio could not be updated since the binary version: %s is not up to date: %s", ver.ClientVersion, ver.TargetVersion)
			return nil
		}

		if isDowngrade(ver) {
			context.Logger.Errorf("Downgrade detected from version: %s to version: %s - finishing...", ver.PilotVersion, ver.TargetVersion)
			return nil
		}

		if !canUpdate(ver) {
			context.Logger.Errorf("Istio could not be updated from version: %s to version: %s - upgrade can be formed by the most one minor version", ver.PilotVersion, ver.TargetVersion)
			return nil
		}

		if isMismatchPresent(ver) {
			context.Logger.Warnf("Istio components version mismatch detected: pilot version: %s, data plane version: %s", ver.PilotVersion, ver.DataPlaneVersion)
		}

		context.Logger.Infof("Istio version was detected on the cluster, updating from %s to %s...", ver.PilotVersion, ver.ClientVersion)

		err = a.performer.Update(context.KubeClient.Kubeconfig(), manifest.Manifest, context.Logger)
		if err != nil {
			return errors.Wrap(err, "Could not update Istio")
		}

		restConfig, err := clientcmd.BuildConfigFromFlags("", context.KubeClient.Kubeconfig())
		if err != nil {
			return err
		}

		kubeClient, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return err
		}

		cfg := istioConfig.IstioProxyConfig{
			ImagePrefix:            istioImagePrefix,
			ImageVersion:          ver.TargetVersion, // TODO: consider adding '-distroless' to the image version under the hood
			RetriesCount:          retriesCount,
			DelayBetweenRetries:   delayBetweenRetries,
			SleepAfterPodDeletion: sleepAfterPodDeletion,
			Kubeclient:            kubeClient,
			Debug:                 true,
			Log:                   context.Logger,
		}

		err = a.istioProxyReset.Run(cfg)
		if err != nil {
			return errors.Wrap(err, "Istio proxy reset error")
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

// shouldInstall checks if istio is already installed
func shouldInstall(version actions.IstioVersion) bool {
	return version.DataPlaneVersion == "" && version.PilotVersion == ""
}

// isClientVersionAcceptable checks if istioctl version is up to date.
func isClientVersionAcceptable(version actions.IstioVersion) bool {
	return version.ClientVersion == version.TargetVersion
}

// canUpdate checks if the update required is different by one minor version.
func canUpdate(version actions.IstioVersion) bool {
	currentVersionSlice := strings.Split(version.PilotVersion, ".")
	targetVersionSlice := strings.Split(version.TargetVersion, ".")

	pilotMinorVersion, err := strconv.Atoi(currentVersionSlice[1])
	if err != nil {
		return false
	}
	targetMinorVersion, err := strconv.Atoi(targetVersionSlice[1])
	if err != nil {
		return false
	}
	if targetVersionSlice[0] == currentVersionSlice[0] && targetMinorVersion-pilotMinorVersion > 1 {
		return false
	}
	return true
}

// isMismatchPresent checks if there is mismatch between Pilot and DataPlane versions.
func isMismatchPresent(version actions.IstioVersion) bool {
	return version.PilotVersion != version.DataPlaneVersion
}

// isDowngrade checks if we are downgrading Istio.
func isDowngrade(version actions.IstioVersion) bool {
	currentVersionSlice := strings.Split(version.PilotVersion, ".")
	targetVersionSlice := strings.Split(version.TargetVersion, ".")

	currentMinorVersion, err := strconv.Atoi(currentVersionSlice[1])
	if err != nil {
		return false
	}
	targetMinorVersion, err := strconv.Atoi(targetVersionSlice[1])
	if err != nil {
		return false
	}
	var currentSubMinorVersion int
	var targetSubMinorVersion int
	if len(currentVersionSlice) == 3 {
		currentSubMinorVersion, err = strconv.Atoi(currentVersionSlice[2])
		if err != nil {
			return false
		}
	}

	if len(targetVersionSlice) == 3 {
		targetSubMinorVersion, err = strconv.Atoi(targetVersionSlice[2])
		if err != nil {
			return false
		}
	}

	if targetVersionSlice[0] == currentVersionSlice[0] {
		if targetMinorVersion > currentMinorVersion {
			return false
		} else if targetMinorVersion == currentMinorVersion {
			if targetSubMinorVersion >= currentSubMinorVersion {
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
