package invoker

import "fmt"

const fallbackComponentReconciler = "base"

type ComponentReconciler struct {
	URL string `json:"url"`
}

type ComponentReconcilersConfig map[string]*ComponentReconciler

type NoFallbackReconcilerDefinedError struct {
}

func (err *NoFallbackReconcilerDefinedError) Error() string {
	return fmt.Sprintf("Fallback component reconciler '%s' is missing: "+
		"check local component reconciler initialization", fallbackComponentReconciler)
}

func IsNoFallbackReconcilerDefinedError(err error) bool {
	_, ok := err.(*NoFallbackReconcilerDefinedError)
	return ok
}
