package invoker

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
)

type NoFallbackReconcilerDefinedError struct {
}

func (err *NoFallbackReconcilerDefinedError) Error() string {
	return fmt.Sprintf("Fallback component reconciler '%s' is missing: "+
		"check local component reconciler initialization", config.FallbackComponentReconciler)
}

func IsNoFallbackReconcilerDefinedError(err error) bool {
	_, ok := err.(*NoFallbackReconcilerDefinedError)
	return ok
}
