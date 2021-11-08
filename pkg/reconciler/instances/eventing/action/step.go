package action

import (
	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

// Steps represents executable steps.
type Steps []Step

// Step represents an executable step.
type Step interface {
	Execute(*service.ActionContext, *zap.SugaredLogger) error
}
