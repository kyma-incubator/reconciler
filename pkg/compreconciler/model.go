package compreconciler

type ReconciliationModel struct {
	Manifest string `json:"manifest"` // TODO remove
	//ChartName    string `json:"chartName"`  // TODO uncomment
	//Configuration    string `json:"configuration"`  // TODO uncomment
	KubeConfig  string `json:"kubeConfig"`
	Version     string `json:"version"`
	CallbackURL string `json:"callbackURL"`
}
