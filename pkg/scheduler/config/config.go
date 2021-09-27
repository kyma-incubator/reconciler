package config

const FallbackComponentReconciler = "base"

type ComponentReconciler struct {
	URL string
}

type SchedulerConfig struct {
	PreComponents []string
	Reconcilers   map[string]ComponentReconciler
}

type Config struct {
	Scheme    string
	Host      string
	Port      int
	Scheduler SchedulerConfig
}
