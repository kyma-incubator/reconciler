package compreconciler

type ReconciliationModel struct {
	Manifest    string `json:...`
	Version     string `json:...`
	CallbackURL string `json:...`
}
