package callback

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
)

type Handler interface {
	Callback(status reconciler.Status) error
}
