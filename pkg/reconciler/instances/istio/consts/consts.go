package consts

const (
	KymaSystem          string = "kyma-system"
	KymaIntegration     string = "kyma-integration"
	KubeSystem          string = "kube-system"
	KymaWarning         string = "kyma-warning"
	NotInIstioMeshLabel string = "pod-not-in-istio-mesh"
	LabelFormat         string = `{"metadata": {"labels": {"%s": "%s"}}}` // Will use KymaWarning and NotInIstioMeshLabel
)
