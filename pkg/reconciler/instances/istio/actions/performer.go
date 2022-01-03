package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/manifest"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	istioConfig "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/types"
)

const (
	istioImagePrefix    = "istio/proxyv2"
	retriesCount        = 5
	delayBetweenRetries = 5 * time.Second
	timeout             = 5 * time.Minute
	interval            = 12 * time.Second
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

type IstioStatus struct {
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

//go:generate mockery --name=IstioPerformer --outpkg=mock --case=underscore
// IstioPerformer performs actions on Istio component on the cluster.
type IstioPerformer interface {

	// Install Istio in given version on the cluster using istioChart.
	Install(kubeConfig, istioChart, version string, logger *zap.SugaredLogger) error

	// PatchMutatingWebhook patches Istio's webhook configuration.
	PatchMutatingWebhook(ctx context.Context, kubeClient kubernetes.Client, logger *zap.SugaredLogger) error

	// Update Istio on the cluster to the targetVersion using istioChart.
	Update(kubeConfig, istioChart, targetVersion string, logger *zap.SugaredLogger) error

	// ResetProxy resets Istio proxy of all Istio sidecars on the cluster. The proxyImageVersion parameter controls the Istio proxy version, it always adds "-distroless" suffix to the provided value.
	ResetProxy(kubeConfig string, proxyImageVersion string, logger *zap.SugaredLogger) error

	// Version reports status of Istio installation on the cluster.
	Version(workspace chart.Factory, branchVersion string, istioChart string, kubeConfig string, logger *zap.SugaredLogger) (IstioStatus, error)

	// Uninstall Istio from the cluster and its corresponding resources, using given Istio version.
	Uninstall(kubeClientSet kubernetes.Client, version string, logger *zap.SugaredLogger) error
}

//CommanderResolver interface implementations must be able to provide istioctl.Commander instances for given istioctl.Version
type CommanderResolver interface {
	//GetCommander function returns istioctl.Commander instance for given istioctl version if supported, returns an error otherwise.
	GetCommander(version istioctl.Version) (istioctl.Commander, error)
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

func (c *DefaultIstioPerformer) Uninstall(kubeClientSet kubernetes.Client, version string, logger *zap.SugaredLogger) error {
	logger.Info("Starting Istio uninstallation...")

	execVersion, err := istioctl.VersionFromString(version)
	if err != nil {
		return errors.Wrap(err, "Error parsing version")
	}

	commander, err := c.resolver.GetCommander(execVersion)
	if err != nil {
		return err
	}

	err = commander.Uninstall(kubeClientSet.Kubeconfig(), logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}
	logger.Info("Istio uninstall triggered")
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
	logger.Info("Istio namespace deleted")
	return nil
}

func (c *DefaultIstioPerformer) Install(kubeConfig, istioChart, version string, logger *zap.SugaredLogger) error {
	logger.Info("Starting Istio installation...")

	execVersion, err := istioctl.VersionFromString(version)
	if err != nil {
		return errors.Wrap(err, "Error parsing version")
	}

	istioOperatorManifest, err := manifest.ExtractIstioOperatorContextFrom(istioChart)
	if err != nil {
		return err
	}

	commander, err := c.resolver.GetCommander(execVersion)
	if err != nil {
		return err
	}

	err = commander.Install(istioOperatorManifest, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	return nil
}

func (c *DefaultIstioPerformer) PatchMutatingWebhook(context context.Context, kubeClient kubernetes.Client, logger *zap.SugaredLogger) error {
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

	err = kubeClient.PatchUsingStrategy(context, "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", patchContentJSON, types.JSONPatchType)
	if err != nil {
		return err
	}

	logger.Infof("Patch has been applied successfully")

	return nil
}

func (c *DefaultIstioPerformer) Update(kubeConfig, istioChart, targetVersion string, logger *zap.SugaredLogger) error {
	logger.Info("Starting Istio update...")

	version, err := istioctl.VersionFromString(targetVersion)
	if err != nil {
		return errors.Wrap(err, "Error parsing version")
	}

	istioOperatorManifest, err := manifest.ExtractIstioOperatorContextFrom(istioChart)
	if err != nil {
		return err
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

func (c *DefaultIstioPerformer) ResetProxy(kubeConfig string, proxyImageVersion string, logger *zap.SugaredLogger) error {
	kubeClient, err := c.provider.RetrieveFrom(kubeConfig, logger)
	if err != nil {
		logger.Error("Could not retrieve KubeClient from Kubeconfig!")
		return err
	}

	cfg := istioConfig.IstioProxyConfig{
		ImagePrefix:         istioImagePrefix,
		ImageVersion:        fmt.Sprintf("%s-distroless", proxyImageVersion),
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
	targetVersion, err := getTargetVersionFromChart(workspace, branchVersion, istioChart)
	if err != nil {
		return IstioStatus{}, errors.Wrap(err, "Target Version could not be obtained")
	}

	version, err := istioctl.VersionFromString(targetVersion)
	if err != nil {
		return IstioStatus{}, errors.Wrap(err, "Error parsing version")
	}

	commander, err := c.resolver.GetCommander(version)
	if err != nil {
		return IstioStatus{}, err
	}

	versionOutput, err := commander.Version(kubeConfig, logger)
	if err != nil {
		return IstioStatus{}, err
	}

	mappedIstioVersion, err := mapVersionToStruct(versionOutput, targetVersion)

	return mappedIstioVersion, err
}

func getTargetVersionFromChart(workspace chart.Factory, branch string, istioChart string) (string, error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return "", err
	}
	helmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return "", err
	}
	return helmChart.Metadata.AppVersion, nil
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

func mapVersionToStruct(versionOutput []byte, targetVersion string) (IstioStatus, error) {
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

	return IstioStatus{
		ClientVersion:    getVersionFromJSON("client", version),
		TargetVersion:    targetVersion,
		PilotVersion:     getVersionFromJSON("pilot", version),
		DataPlaneVersion: getVersionFromJSON("dataPlane", version),
	}, nil
}
