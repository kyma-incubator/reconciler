package compreconciler

type Reconciliation struct {
	Component     string          `json:"component"`
	Namespace     string          `json:"namespace"`
	Version       string          `json:"version"`
	Profile       string          `json:"profile"`
	Configuration []Configuration `json:"configuration"`
	Kubeconfig    string          `json:"kubeconfig"`
	CallbackURL   string          `json:"callbackURL"`
}

type Configuration struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
