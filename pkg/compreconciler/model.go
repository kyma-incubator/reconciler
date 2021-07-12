package compreconciler

type ReconciliationModel struct {
	Manifest    string `json:"manifest"`
	KubeConfig  string `json:"kubeConfig"`
	Version     string `json:"version"`
	CallbackURL string `json:"callbackURL"`
}
