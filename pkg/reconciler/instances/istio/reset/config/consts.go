package config

const (
	KymaWarning string = "kyma-warning"
	LabelWarning string = "pod not in istio mesh"
	LabelFormat  string = `{"metadata": {"labels": {"kyma-warning": "%s"}}}`
)