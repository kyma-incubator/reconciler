package actions

import (
	"bytes"
	"encoding/json"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

const (
	istioOperatorKind = "IstioOperator"
)

type VersionType string

const (
	client    VersionType = "client"
	pilot                 = "pilot"
	dataPlane             = "dataPlane"
)

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

// IstioVersion TODO
type IstioVersion struct {
	ClientVersion    string
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

//go:generate mockery -name=IstioPerformer
// IstioPerformer performs actions on Istio component on the cluster.
type IstioPerformer interface {

	// Install Istio on the cluster.
	Install(kubeConfig, manifest string, logger *zap.SugaredLogger, commander istioctl.Commander) error

	// PatchMutatingWebhook configuration.
	PatchMutatingWebhook(kubeClient kubernetes.Client, logger *zap.SugaredLogger) error

	// Update Istio on the cluster.
	Update(kubeConfig, manifest string, logger *zap.SugaredLogger, commander istioctl.Commander) error

	// Version of Istio on the cluster.
	Version(kubeConfig string, logger *zap.SugaredLogger, commander istioctl.Commander) (IstioVersion, error)
}

// DefaultIstioPerformer provides a default implementation of IstioPerformer.
type DefaultIstioPerformer struct {
	commander istioctl.Commander
}

// NewDefaultIstioPerformer creates a new instance of the DefaultIstioPerformer.
func NewDefaultIstioPerformer(commander istioctl.Commander) *DefaultIstioPerformer {
	return &DefaultIstioPerformer{
		commander: commander,
	}
}

func (c *DefaultIstioPerformer) Install(kubeConfig, manifest string, logger *zap.SugaredLogger, commander istioctl.Commander) error {
	istioOperator, err := extractIstioOperatorContextFrom(manifest)
	if err != nil {
		return err
	}

	logger.Info("Starting Istio installation...")

	err = commander.Install(istioOperator, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	return nil
}

func (c *DefaultIstioPerformer) PatchMutatingWebhook(kubeClient kubernetes.Client, logger *zap.SugaredLogger) error {
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

func (c *DefaultIstioPerformer) Update(kubeConfig, manifest string, logger *zap.SugaredLogger, commander istioctl.Commander) error {
	istioOperator, err := extractIstioOperatorContextFrom(manifest)
	if err != nil {
		return err
	}

	logger.Info("Starting Istio update...")

	err = commander.Upgrade(istioOperator, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	logger.Info("Istio has been updated successfully")

	return nil
}

func (c *DefaultIstioPerformer) Version(kubeConfig string, logger *zap.SugaredLogger, commander istioctl.Commander) (IstioVersion, error) {
	version, err := c.commander.Version(kubeConfig, logger)
	if err != nil {
		return IstioVersion{}, err
	}

	mappedIstioVersion, err := mapVersionToStruct(version, logger)

	return mappedIstioVersion, err
}

func extractIstioOperatorContextFrom(manifest string) (string, error) {
	unstructs, err := kubeclient.ToUnstructured([]byte(manifest), true)
	if err != nil {
		return "", err
	}

	for _, unstruct := range unstructs {
		if unstruct.GetKind() != istioOperatorKind {
			continue
		}

		unstructBytes, err := unstruct.MarshalJSON()
		if err != nil {
			return "", nil
		}

		return string(unstructBytes), nil
	}

	return "", errors.New("Istio Operator definition could not be found in manifest")
}

func getVersionFromJSON(versionType VersionType, json IstioVersionOutput) string {
	switch versionType {
	case "client":
		return json.ClientVersion.Version
	case "pilot":
		if len(json.MeshVersion) > 0 {
			return json.MeshVersion[0].Info.Version
		} else {
			return ""
		}
	case "dataPlane":
		if len(json.DataPlaneVersion) > 0 {
			return json.DataPlaneVersion[0].IstioVersion
		} else {
			return ""
		}
	default:
		return ""
	}
}

func mapVersionToStruct(raw []byte, logger *zap.SugaredLogger) (IstioVersion, error) {
	//	If raw is empty
	if len(raw) == 0 {
		return IstioVersion{}, errors.New("The result of the version command is empty!")
	}

	// Remove additional text not part of the json output
	if index := bytes.IndexRune(raw, '{'); index != 0 {
		raw = raw[bytes.IndexRune(raw, '{'):]
	}

	var version IstioVersionOutput
	// Map the json output to the IstioVersionOutput struct
	err := json.Unmarshal(raw, &version)

	if err != nil {
		return IstioVersion{}, err
	}

	return IstioVersion{
		ClientVersion:    getVersionFromJSON("client", version),
		PilotVersion:     getVersionFromJSON("pilot", version),
		DataPlaneVersion: getVersionFromJSON("dataPlane", version),
	}, nil
}
