package scheduler

//ComponentReconciler is the model used to describe the component reconciler configuration
type ComponentReconciler struct {
	URL string `json:"url"`
}

type ComponentReconcilersConfig map[string]*ComponentReconciler

type MothershipReconcilerConfig struct {
	Host          string
	Port          int
	CrdComponents []string
	PreComponents []string
}
