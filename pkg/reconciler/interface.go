package reconciler

type ComponentReconciler interface {
	Reconciler(manifest *Manifest, kubeConfig, callbackURL string) error
}

type Manifest struct {
	Manifest string
	Version  string
}

type ComponentReconciliationStatus string

const (
	Failed  ComponentReconciliationStatus = "failed"
	Error   ComponentReconciliationStatus = "error"
	Running ComponentReconciliationStatus = "running"
	Success ComponentReconciliationStatus = "success"
)
