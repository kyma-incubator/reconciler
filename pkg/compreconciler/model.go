package compreconciler

type Reconciliation struct {
	Component     string          `json:"component"`
	Namespace     string          `json:"namespace"`
	Version       string          `json:"version"`
	Configuration []Configuration `json:"configuration"`
	KubeConfig    string          `json:"kubeConfig"`
	CallbackURL   string          `json:"callbackURL"`
}

type Configuration struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
