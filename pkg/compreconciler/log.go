package compreconciler

import "go.uber.org/zap"
import log "github.com/kyma-incubator/reconciler/pkg/logger"

func newLogger(debug bool) *zap.Logger {
	logger, err := log.NewLogger(debug)
	if err != nil {
		logger = zap.NewNop()
	}
	return logger
}
