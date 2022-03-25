package invoker

import (
	"fmt"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
)

type NoFallbackReconcilerDefinedError struct {
}

func (err *NoFallbackReconcilerDefinedError) Error() string {
	return fmt.Sprintf("Fallback component reconciler '%s' is missing: "+
		"check local component reconciler initialization", config.FallbackComponentReconciler)
}

func IsNoFallbackReconcilerDefinedError(err error) bool {
	var ok bool
	rErr, isRetryErr := err.(retry.Error)
	if isRetryErr {
		for _, err := range rErr.WrappedErrors() {
			_, ok = err.(*NoFallbackReconcilerDefinedError)
			break
		}
	} else {
		_, ok = err.(*NoFallbackReconcilerDefinedError)
	}
	return ok
}
