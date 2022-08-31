package config

const (
	KymaWarning         string = "kyma-warning"
	NotInIstioMeshLabel string = "pod-not-in-istio-mesh"
	LabelFormat         string = `{"metadata": {"labels": {"%s": "%s"}}}` // Will use KymaWarning and NotInIstioMeshLabel
)
