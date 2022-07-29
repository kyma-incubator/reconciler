package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"path/filepath"
	"reflect"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/helpers"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/manifest"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgo "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	istioConfig "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	helmChart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

const (
	retriesCount        = 5
	delayBetweenRetries = 5 * time.Second
	timeout             = 5 * time.Minute
	interval            = 12 * time.Second
)

type VersionType string

type IstioStatus struct {
	ClientVersion    helpers.HelperVersion
	TargetVersion    helpers.HelperVersion
	PilotVersion     *helpers.HelperVersion
	DataPlaneVersion *helpers.HelperVersion
}

type IstioVersionOutput struct {
	ClientVersion    *ClientVersion      `json:"clientVersion"`
	MeshVersion      []*MeshComponent    `json:"meshVersion,omitempty"`
	DataPlaneVersion []*DataPlaneVersion `json:"dataPlaneVersion,omitempty"`
}

type ClientVersion struct {
	Version string `json:"version"`
}

type MeshComponent struct {
	Component string    `json:"Component,omitempty"`
	Info      *MeshInfo `json:"Info,omitempty"`
}

type MeshInfo struct {
	Version string `json:"version,omitempty"`
}

type DataPlaneVersion struct {
	IstioVersion string `json:"IstioVersion,omitempty"`
}

type chartValues struct {
	Global struct {
		SidecarMigration bool `json:"sidecarMigration"`
		Images           struct {
			IstioPilot struct {
				Version string `json:"version"`
			} `json:"istio_pilot"`
			IstioProxyV2 struct {
				Directory             string `json:"directory"`
				ContainerRegistryPath string `json:"containerRegistryPath"`
			} `json:"istio_proxyv2"`
		} `json:"images"`
	} `json:"global"`
}

//go:generate mockery --name=IstioPerformer --outpkg=mock --case=underscore
// IstioPerformer performs actions on Istio component on the cluster.
type IstioPerformer interface {

	// Install Istio in given version on the cluster using istioChart.
	Install(kubeConfig, istioChart string, version helpers.HelperVersion, logger *zap.SugaredLogger) error

	// PatchMutatingWebhook patches Istio's webhook configuration.
	PatchMutatingWebhook(context context.Context, kubeClient kubernetes.Client, workspace chart.Factory, branchVersion string, istioChart string, logger *zap.SugaredLogger) error

	// LabelNamespaces labels all namespaces with enabled istio sidecar migration.
	LabelNamespaces(context context.Context, kubeClient kubernetes.Client, workspace chart.Factory, branchVersion string, istioChart string, logger *zap.SugaredLogger) error

	// Update Istio on the cluster to the targetVersion using istioChart.
	Update(kubeConfig, istioChart string, targetVersion helpers.HelperVersion, logger *zap.SugaredLogger) error

	// ResetProxy resets Istio proxy of all Istio sidecars on the cluster. The proxyImageVersion parameter controls the Istio proxy version.
	ResetProxy(context context.Context, kubeConfig string, proxyImageVersion helpers.HelperVersion, logger *zap.SugaredLogger) error

	// Version reports status of Istio installation on the cluster.
	Version(workspace chart.Factory, branchVersion string, istioChart string, kubeConfig string, logger *zap.SugaredLogger) (IstioStatus, error)

	// Uninstall Istio from the cluster and its corresponding resources, using given Istio version.
	Uninstall(kubeClientSet kubernetes.Client, version helpers.HelperVersion, logger *zap.SugaredLogger) error
}

// CommanderResolver interface implementations must be able to provide istioctl.Commander instances for given istioctl.Version
type CommanderResolver interface {
	// GetCommander function returns istioctl.Commander instance for given istioctl version if supported, returns an error otherwise.
	GetCommander(version helpers.HelperVersion) (istioctl.Commander, error)
}

// DefaultIstioPerformer provides a default implementation of IstioPerformer.
// It uses istioctl binary to do it's job. It delegates the job of finding proper istioctl binary for given operation to the configured CommandResolver.
type DefaultIstioPerformer struct {
	resolver        CommanderResolver
	istioProxyReset proxy.IstioProxyReset
	provider        clientset.Provider
}

// NewDefaultIstioPerformer creates a new instance of the DefaultIstioPerformer.
func NewDefaultIstioPerformer(resolver CommanderResolver, istioProxyReset proxy.IstioProxyReset, provider clientset.Provider) *DefaultIstioPerformer {
	return &DefaultIstioPerformer{resolver, istioProxyReset, provider}
}

func (c *DefaultIstioPerformer) Uninstall(kubeClientSet kubernetes.Client, version helpers.HelperVersion, logger *zap.SugaredLogger) error {
	logger.Debug("Starting Istio uninstallation...")

	commander, err := c.resolver.GetCommander(version)
	if err != nil {
		return err
	}

	err = commander.Uninstall(kubeClientSet.Kubeconfig(), logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}
	logger.Debug("Istio uninstall triggered")
	kubeClient, err := kubeClientSet.Clientset()
	if err != nil {
		return err
	}

	policy := metav1.DeletePropagationForeground
	err = kubeClient.CoreV1().Namespaces().Delete(context.TODO(), "istio-system", metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
	if err != nil {
		return err
	}
	logger.Debug("Istio namespace deleted")
	return nil
}

func (c *DefaultIstioPerformer) Install(kubeConfig, istioChart string, version helpers.HelperVersion, logger *zap.SugaredLogger) error {
	logger.Debug("Starting Istio installation...")

	istioOperatorManifest, err := manifest.ExtractIstioOperatorContextFrom(istioChart)
	if err != nil {
		return err
	}

	commander, err := c.resolver.GetCommander(version)
	if err != nil {
		return err
	}

	err = commander.Install(istioOperatorManifest, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}
	logger.Infof("Istio in version %s successfully installed", version)
	return nil
}

func (c *DefaultIstioPerformer) PatchMutatingWebhook(context context.Context, kubeClient kubernetes.Client, workspace chart.Factory, branchVersion string, istioChart string, logger *zap.SugaredLogger) error {
	logger.Debugf("Patching mutating webhook")
	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	const istioWebHookConfName = "istio-revision-tag-default"
	webhookNameToChange := "auto.sidecar-injector.istio.io"
	requiredLabelSelector := metav1.LabelSelectorRequirement{
		Key:      "gardener.cloud/purpose",
		Operator: "NotIn",
		Values:   []string{"kube-system"},
	}

	sidecarMigrationEnabled, sidecarMigrationIsSet, err := isSidecarMigrationEnabled(workspace, branchVersion, istioChart)
	if err != nil {
		return err
	}
	if sidecarMigrationEnabled || !sidecarMigrationIsSet {
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			whConf, err := c.selectWebhookConf(context, istioWebHookConfName, clientSet)
			if err != nil {
				return err
			}
			err = c.addNamespaceSelectorIfNotPresent(whConf, webhookNameToChange, requiredLabelSelector)
			if err != nil {
				return err
			}
			_, err = clientSet.AdmissionregistrationV1().
				MutatingWebhookConfigurations().
				Update(context, whConf, metav1.UpdateOptions{})
			return err
		})
		if err != nil {
			return err
		}

		logger.Debugf("Patch has been applied successfully")
		return nil
	}

	logger.Debugf("Sidecar migration is disabled or not set, skipping mutating webhook patch")
	return nil
}

func (c *DefaultIstioPerformer) LabelNamespaces(context context.Context, kubeClient kubernetes.Client, workspace chart.Factory, branchVersion string, istioChart string, logger *zap.SugaredLogger) error {
	logger.Debugf("Labeling namespaces with istio-injection: enabled")
	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	labelPatch := `{"metadata": {"labels": {"istio-injection": "enabled"}}}`

	sidecarMigrationEnabled, sidecarMigrationIsSet, err := isSidecarMigrationEnabled(workspace, branchVersion, istioChart)
	if err != nil {
		return err
	}
	if sidecarMigrationEnabled && sidecarMigrationIsSet {
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			namespaces, err := clientSet.CoreV1().Namespaces().List(context, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, namespace := range namespaces.Items {
				_, isIstioInjectionSet := namespace.Labels["istio-injection"]
				if !isIstioInjectionSet && namespace.ObjectMeta.Name != "kube-system" {
					logger.Debugf("Patching namespace %s with label istio-injection: enabled", namespace.ObjectMeta.Name)
					_, err = clientSet.CoreV1().Namespaces().Patch(context, namespace.ObjectMeta.Name, types.MergePatchType, []byte(labelPatch), metav1.PatchOptions{})
				}
			}
			return err
		})
		if err != nil {
			return err
		}

		logger.Debugf("Namespaces have been labeled successfully")
	} else {
		logger.Debugf("Sidecar migration is disabled or it is not set, skipping labeling namespaces")
	}

	return nil
}

func (c *DefaultIstioPerformer) addNamespaceSelectorIfNotPresent(whConf *v1.MutatingWebhookConfiguration, webhookNameToChange string, requiredLabelSelector metav1.LabelSelectorRequirement) error {
	for i := range whConf.Webhooks {
		if whConf.Webhooks[i].Name == webhookNameToChange {
			matchExpressions := whConf.Webhooks[i].NamespaceSelector.MatchExpressions
			var hasRequiredLabel bool
			for j := range matchExpressions {
				if hasRequiredLabel = reflect.DeepEqual(matchExpressions[j], requiredLabelSelector); hasRequiredLabel {
					break
				}
			}
			if !hasRequiredLabel {
				matchExpressions = append(matchExpressions, requiredLabelSelector)
				whConf.Webhooks[i].NamespaceSelector.MatchExpressions = matchExpressions
			}
			return nil
		}
	}
	return fmt.Errorf("could not find webhook %s in WebhookConfiguration %s", webhookNameToChange, whConf.Name)
}

func (c *DefaultIstioPerformer) selectWebhookConf(context context.Context, webhookConfName string, clientSet clientgo.Interface) (wh *v1.MutatingWebhookConfiguration, err error) {

	wh, err = clientSet.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context, webhookConfName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "MutatingWebhookConfigurations could not be selected from candidates")
	}
	return
}

func (c *DefaultIstioPerformer) Update(kubeConfig, istioChart string, targetVersion helpers.HelperVersion, logger *zap.SugaredLogger) error {
	logger.Debug("Starting Istio update...")

	istioOperatorManifest, err := manifest.ExtractIstioOperatorContextFrom(istioChart)
	if err != nil {
		return err
	}

	commander, err := c.resolver.GetCommander(targetVersion)
	if err != nil {
		return err
	}

	err = commander.Upgrade(istioOperatorManifest, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	logger.Infof("Istio has been updated successfully to version %s", targetVersion)

	return nil
}

func (c *DefaultIstioPerformer) ResetProxy(context context.Context, kubeConfig string, proxyImageVersion helpers.HelperVersion, logger *zap.SugaredLogger) error {
	kubeClient, err := c.provider.RetrieveFrom(kubeConfig, logger)
	if err != nil {
		logger.Error("Could not retrieve KubeClient from Kubeconfig!")
		return err
	}

	cfg := istioConfig.IstioProxyConfig{
		Context:             context,
		ImageVersion:        proxyImageVersion,
		RetriesCount:        retriesCount,
		DelayBetweenRetries: delayBetweenRetries,
		Timeout:             timeout,
		Interval:            interval,
		Kubeclient:          kubeClient,
		Debug:               false,
		Log:                 logger,
	}

	err = c.istioProxyReset.Run(cfg)
	if err != nil {
		return errors.Wrap(err, "Istio proxy reset error")
	}

	return nil
}

func (c *DefaultIstioPerformer) Version(workspace chart.Factory, branchVersion string, istioChart string, kubeConfig string, logger *zap.SugaredLogger) (IstioStatus, error) {
	targetVersion, err := getTargetVersionFromIstioChart(workspace, branchVersion, istioChart, logger)
	if err != nil {
		return IstioStatus{}, errors.Wrap(err, "Target Version could not be found")
	}

	targetLibrary, err := getTargetProxyV2LibraryFromIstioChart(workspace, branchVersion, istioChart, logger)
	if err != nil {
		return IstioStatus{}, errors.Wrap(err, "Target Prefix could not be found")
	}

	version, err := semver.NewVersion(targetVersion)
	if err != nil {
		return IstioStatus{}, errors.Wrap(err, "Error parsing version")
	}

	helperVersion := helpers.HelperVersion{Library: targetLibrary, Tag: *version}

	commander, err := c.resolver.GetCommander(helperVersion)
	if err != nil {
		return IstioStatus{}, err
	}

	versionOutput, err := commander.Version(kubeConfig, logger)
	if err != nil {
		return IstioStatus{}, err
	}

	mappedIstioVersion, err := mapVersionToStruct(versionOutput, targetVersion, targetLibrary, logger)

	return mappedIstioVersion, err
}

func getTargetVersionFromIstioChart(workspace chart.Factory, branch string, istioChart string, logger *zap.SugaredLogger) (string, error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return "", err
	}

	istioHelmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return "", err
	}

	pilotVersion, err := getTargetVersionFromPilotInChartValues(istioHelmChart)
	if err != nil {
		return "", err
	}

	if pilotVersion != "" {
		logger.Debugf("Resolved target Istio version: %s from values", pilotVersion)
		return pilotVersion, nil
	}

	chartVersion := getTargetVersionFromVersionInChartDefinition(istioHelmChart)
	if chartVersion != "" {
		logger.Debugf("Resolved target Istio version: %s from Chart definition", chartVersion)
		return chartVersion, nil
	}

	return "", errors.New("Target Istio version could not be found neither in Chart.yaml nor in helm values")
}

func getTargetProxyV2LibraryFromIstioChart(workspace chart.Factory, branch string, istioChart string, logger *zap.SugaredLogger) (string, error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return "", err
	}

	istioHelmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return "", err
	}

	istioValuesRegistryPath, istioValuesDirectory, err := getTargetProxyV2PrefixFromIstioValues(istioHelmChart)
	if err != nil {
		return "", errors.New("Could not resolve target proxyV2 Istio prefix from values")
	}

	prefix := fmt.Sprintf("%s/%s", istioValuesRegistryPath, istioValuesDirectory)
	logger.Debugf("Resolved target Istio prefix: %s from istio values.yaml", prefix)
	return prefix, nil
}

func getTargetVersionFromVersionInChartDefinition(helmChart *helmChart.Chart) string {
	return helmChart.Metadata.Version
}

func getTargetVersionFromPilotInChartValues(helmChart *helmChart.Chart) (string, error) {
	mapAsJSON, err := json.Marshal(helmChart.Values)
	if err != nil {
		return "", err
	}

	var chartValues chartValues
	err = json.Unmarshal(mapAsJSON, &chartValues)
	if err != nil {
		return "", err
	}

	return chartValues.Global.Images.IstioPilot.Version, nil
}

func getTargetProxyV2PrefixFromIstioValues(istioHelmChart *helmChart.Chart) (string, string, error) {
	mapAsJSON, err := json.Marshal(istioHelmChart.Values)
	if err != nil {
		return "", "", err
	}
	var chartValues chartValues

	err = json.Unmarshal(mapAsJSON, &chartValues)
	if err != nil {
		return "", "", err
	}
	containerRegistryPath := chartValues.Global.Images.IstioProxyV2.ContainerRegistryPath
	directory := chartValues.Global.Images.IstioProxyV2.Directory

	return containerRegistryPath, directory, nil
}

func getVersionFromJSON(versionType VersionType, json IstioVersionOutput) (*helpers.HelperVersion, error) {
	switch versionType {
	case "client":
		version, err := semver.NewVersion(json.ClientVersion.Version)
		if err != nil {
			return nil, err
		}
		return &helpers.HelperVersion{Library: "", Tag: *version}, nil
	case "pilot":
		if len(json.MeshVersion) > 0 {
			return helpers.NewHelperVersionFrom(json.MeshVersion[0].Info.Version)
		}
		return nil, nil
	case "dataPlane":
		if len(json.DataPlaneVersion) > 0 {
			return helpers.NewHelperVersionFrom(json.DataPlaneVersion[0].IstioVersion)
		}
		return nil, nil
	default:
		return nil, errors.New("no version type specified")
	}
}

func mapVersionToStruct(versionOutput []byte, targetTag string, targetLibrary string, logger *zap.SugaredLogger) (IstioStatus, error) {
	if len(versionOutput) == 0 {
		return IstioStatus{}, errors.New("the result of the version command is empty")
	}

	if index := bytes.IndexRune(versionOutput, '{'); index != 0 {
		versionOutput = versionOutput[bytes.IndexRune(versionOutput, '{'):]
	}

	var version IstioVersionOutput
	err := json.Unmarshal(versionOutput, &version)

	if err != nil {
		return IstioStatus{}, err
	}

	clientVersion, err := getVersionFromJSON("client", version)
	if err != nil {
		return IstioStatus{}, err
	}

	tag, err := semver.NewVersion(targetTag)
	if err != nil {
		return IstioStatus{}, err
	}
	targetVersion := helpers.HelperVersion{Library: targetLibrary, Tag: *tag}

	pilotVersion, err := getVersionFromJSON("pilot", version)
	if err != nil {
		logger.Infof("Pilot Istio version wasn't found on cluster, %s", err)
	} else {
		logger.Infof("Istio pilot was found on cluster in version %s", pilotVersion.String())
	}
	dataPlaneVersion, err := getVersionFromJSON("dataPlane", version)
	if err != nil {
		logger.Infof("Data plane Istio version wasn't found on cluster, %s", err)
	} else {
		logger.Infof("Data plane Istio version was found on cluster in version %s", dataPlaneVersion.String())
	}

	return IstioStatus{
		ClientVersion:    *clientVersion,
		TargetVersion:    targetVersion,
		PilotVersion:     pilotVersion,
		DataPlaneVersion: dataPlaneVersion}, nil
}

func isSidecarMigrationEnabled(workspace chart.Factory, branch string, istioChart string) (option bool, isSet bool, err error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return false, false, err
	}

	istioHelmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return false, false, err
	}

	mapAsJSON, err := json.Marshal(istioHelmChart.Values)
	if err != nil {
		return false, false, err
	}
	var chartValues chartValues

	err = json.Unmarshal(mapAsJSON, &chartValues)
	if err != nil {
		return false, false, err
	}
	option = chartValues.Global.SidecarMigration

	isSet = false
	var rawValues map[string]map[string]interface{}
	err = json.Unmarshal(mapAsJSON, &rawValues)
	if err != nil {
		return false, false, err
	}
	if global, isGlobalSet := rawValues["global"]; isGlobalSet {
		if _, isSidecarMigrationSet := global["sidecarMigration"]; isSidecarMigrationSet {
			isSet = true
		}
	}

	return option, isSet, nil
}
