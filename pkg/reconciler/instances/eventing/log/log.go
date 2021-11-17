package log

import (
	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	// KeyError is used as a named key for a log message with error.
	KeyError = "error"

	// KeyResult is used as a named key for a log message with result.
	KeyResult = "result"

	// KeyReason is used as a named key for a log message with reason.
	KeyReason = "reason"

	// KeyStep is used as a named key for a log message with step.
	KeyStep = "step"

	// keyAction is used as a named key for a log message with action.
	keyAction = "action"

	// keyVersion is used as a named key for a log message with version.
	keyVersion = "version"

	// ValueFail is used as a value for a log message with failure.
	ValueFail = "fail"

	// ValueSuccess is used as a value for a log message with success.
	ValueSuccess = "success"
)

// New returns a new logger.
func New() *zap.SugaredLogger {
	return logger.NewLogger(false)
}

// ContextLogger returns a decorated logger with the given context and LoggerOpts.
func ContextLogger(context *service.ActionContext, opts ...LoggerOpt) *zap.SugaredLogger {
	length := 2*len(opts) + 2
	args := make([]interface{}, 0, length)

	// append log pairs
	for _, opt := range opts {
		pair := opt()
		args = append(args, pair[0], pair[1])
	}

	// always append version
	args = append(args, keyVersion, context.Task.Version)

	return context.Logger.With(args...)
}

// pair represents a log key/value pair.
type pair [2]string

// LoggerOpt represents a function that returns a log pair instance when executed.
type LoggerOpt func() pair

// WithAction returns a LoggerOpt for the given action.
func WithAction(action string) LoggerOpt {
	return func() pair {
		return pair{keyAction, action}
	}
}
