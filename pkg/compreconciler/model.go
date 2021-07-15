package compreconciler

type Reconciliation struct {
	Component     string          `json:"component"`
	Namespace     string          `json:"namespace"`
	Version       string          `json:"version"`
	Profile       string          `json:"profile"`
	Configuration []Configuration `json:"configuration"`
	Kubeconfig    string          `json:"kubeconfig"`
	//CallbackURL is mandatory when component-reconciler runs in separate process
	CallbackURL string `json:"callbackURL"`
	//CallbackFct has to be set when component-reconciler runs embedded
	CallbackFct func(status Status) error
}

type Configuration struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
