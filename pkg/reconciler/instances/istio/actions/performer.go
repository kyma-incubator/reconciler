package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	istioConfig "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	reconcilerKubeClient "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/types"
)

const (
	istioImagePrefix      = "istio/proxyv2"
	retriesCount          = 5
	delayBetweenRetries   = 10
	sleepAfterPodDeletion = 10
)

type VersionType string

type webhookPatchJSON struct {
	Op    string                `json:"op"`
	Path  string                `json:"path"`
	Value webhookPatchJSONValue `json:"value"`
}

type webhookPatchJSONValue struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

type IstioVersion struct {
	ClientVersion    string
	TargetVersion    string
	PilotVersion     string
	DataPlaneVersion string
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

//go:generate mockery --name=CommanderResolver --outpkg=mock --case=underscore
type CommanderResolver interface {
	GetCommander(version istioctl.Version) (istioctl.Commander, error)
}

//go:generate mockery --name=IstioPerformer --outpkg=mock --case=underscore
// IstioPerformer performs actions on Istio component on the cluster.
// It can switch between istio versions automatically.
type IstioPerformer interface {

	// Install Istio on the cluster. Uses istioctl version that matches the provided version.
	Install(kubeConfig, istioOperatormanifest, version string, logger *zap.SugaredLogger) error

	// PatchMutatingWebhook configuration.
	PatchMutatingWebhook(kubeClient reconcilerKubeClient.Client, logger *zap.SugaredLogger) error

	// Update Istio on the cluster. Uses istioctl version that matches the provided version.
	Update(kubeConfig, istioOperatorManifest, version string, logger *zap.SugaredLogger) error

	// ResetProxy of all Istio sidecars on the cluster. Uses istioctl version that matches provided version.TargetVersion
	ResetProxy(kubeConfig string, version IstioVersion, logger *zap.SugaredLogger) error

	// Version of Istio on the cluster. Uses istioctl version that matches version of the istioChart
	Version(workspace workspace.Factory, branchVersion string, istioChart string, kubeConfig string, logger *zap.SugaredLogger) (IstioVersion, error)
}

// DefaultIstioPerformer provides a default implementation of IstioPerformer.
type DefaultIstioPerformer struct {
	resolver        CommanderResolver
	istioProxyReset proxy.IstioProxyReset
	provider        clientset.Provider
}

// NewDefaultIstioPerformer creates a new instance of the DefaultIstioPerformer.
func NewDefaultIstioPerformer(resolver CommanderResolver, istioProxyReset proxy.IstioProxyReset, provider clientset.Provider) *DefaultIstioPerformer {
	return &DefaultIstioPerformer{resolver, istioProxyReset, provider}
}

func (c *DefaultIstioPerformer) Install(kubeConfig, istioOperatorManifest, semver string, logger *zap.SugaredLogger) error {
	logger.Info("Starting Istio installation...")

	version, err := istioctl.VersionFromString(semver)
	if err != nil {
		return errors.Wrap(err, "Error parsing version")
	}

	commander, err := c.resolver.GetCommander(version)
	if err != nil {
		return err
	}

	err = commander.Install(istioOperatorManifest, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	return nil
}

func (c *DefaultIstioPerformer) PatchMutatingWebhook(kubeClient reconcilerKubeClient.Client, logger *zap.SugaredLogger) error {
	patchContent := []webhookPatchJSON{{
		Op:   "add",
		Path: "/webhooks/4/namespaceSelector/matchExpressions/-",
		Value: webhookPatchJSONValue{
			Key:      "gardener.cloud/purpose",
			Operator: "NotIn",
			Values: []string{
				"kube-system",
			},
		},
	}}

	patchContentJSON, err := json.Marshal(patchContent)
	if err != nil {
		return err
	}

	logger.Info("Patching istio-sidecar-injector MutatingWebhookConfiguration...")

	err = kubeClient.PatchUsingStrategy("MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", patchContentJSON, types.JSONPatchType)
	if err != nil {
		return err
	}

	logger.Infof("Patch has been applied successfully")

	return nil
}

func (c *DefaultIstioPerformer) Update(kubeConfig, istioOperatorManifest, semver string, logger *zap.SugaredLogger) error {
	logger.Info("Starting Istio update...")

	version, err := istioctl.VersionFromString(semver)
	if err != nil {
		return errors.Wrap(err, "Error parsing version")
	}

	commander, err := c.resolver.GetCommander(version)
	if err != nil {
		return err
	}

	err = commander.Upgrade(istioOperatorManifest, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	logger.Info("Istio has been updated successfully")

	return nil
}

func (c *DefaultIstioPerformer) ResetProxy(kubeConfig string, version IstioVersion, logger *zap.SugaredLogger) error {
	kubeClient, err := c.provider.RetrieveFrom(kubeConfig, logger)
	if err != nil {
		return err
	}

	cfg := istioConfig.IstioProxyConfig{
		ImagePrefix:           istioImagePrefix,
		ImageVersion:          fmt.Sprintf("%s-distroless", version.TargetVersion),
		RetriesCount:          retriesCount,
		DelayBetweenRetries:   delayBetweenRetries,
		SleepAfterPodDeletion: sleepAfterPodDeletion,
		Kubeclient:            kubeClient,
		Debug:                 false,
		Log:                   logger,
	}

	err = c.istioProxyReset.Run(cfg)
	if err != nil {
		return errors.Wrap(err, "Istio proxy reset error")
	}

	return nil
}

func (c *DefaultIstioPerformer) Version(workspace workspace.Factory, branchVersion string, istioChart string, kubeConfig string, logger *zap.SugaredLogger) (IstioVersion, error) {
	targetVersion, err := getTargetVersionFromChart(workspace, branchVersion, istioChart)
	if err != nil {
		return IstioVersion{}, errors.Wrap(err, "Target Version could not be obtained")
	}

	version, err := istioctl.VersionFromString(targetVersion)
	if err != nil {
		return IstioVersion{}, errors.Wrap(err, "Error parsing version")
	}

	commander, err := c.resolver.GetCommander(version)
	if err != nil {
		return IstioVersion{}, err
	}

	versionOutput, err := commander.Version(kubeConfig, logger)
	if err != nil {
		return IstioVersion{}, err
	}

	mappedIstioVersion, err := mapVersionToStruct(versionOutput, targetVersion)

	return mappedIstioVersion, err
}

func getTargetVersionFromChart(workspace workspace.Factory, branch string, istioChart string) (string, error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return "", err
	}
	chart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return "", err
	}
	return chart.Metadata.AppVersion, nil
}

func getVersionFromJSON(versionType VersionType, json IstioVersionOutput) string {
	switch versionType {
	case "client":
		return json.ClientVersion.Version
	case "pilot":
		if len(json.MeshVersion) > 0 {
			return json.MeshVersion[0].Info.Version
		}
		return ""
	case "dataPlane":
		if len(json.DataPlaneVersion) > 0 {
			return json.DataPlaneVersion[0].IstioVersion
		}
		return ""
	default:
		return ""
	}
}

func mapVersionToStruct(versionOutput []byte, targetVersion string) (IstioVersion, error) {
	if len(versionOutput) == 0 {
		return IstioVersion{}, errors.New("the result of the version command is empty")
	}

	if index := bytes.IndexRune(versionOutput, '{'); index != 0 {
		versionOutput = versionOutput[bytes.IndexRune(versionOutput, '{'):]
	}

	var version IstioVersionOutput
	err := json.Unmarshal(versionOutput, &version)

	if err != nil {
		return IstioVersion{}, err
	}

	return IstioVersion{
		ClientVersion:    getVersionFromJSON("client", version),
		TargetVersion:    targetVersion,
		PilotVersion:     getVersionFromJSON("pilot", version),
		DataPlaneVersion: getVersionFromJSON("dataPlane", version),
	}, nil
}
