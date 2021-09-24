package config

//ComponentReconciler is the model used to describe the component reconciler configuration
type ComponentReconciler struct {
	URL string `json:"url"`
}

type ComponentReconcilersConfig map[string]*ComponentReconciler

type MothershipReconcilerConfig struct {
	Scheme        string
	Host          string
	Port          int
	PreComponents []string
}
