package logger

import (
	"fmt"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	loggerInstance *zap.SugaredLogger
	once           sync.Once
	err            error
	corrID         string
)

func NewLogger(correlationID string, debug bool) (*zap.SugaredLogger, error) {
	if correlationID != "" {
		corrID = correlationID
	}
	if corrID == "" {
		return nil, fmt.Errorf("Correlation ID is empty. Logger cannot be created without the correlation ID.")
	}

	once.Do(func() { // atomic, does not allow repeating
		loggerInstance, err = logger.NewLogger(debug)
		if loggerInstance != nil {
			loggerInstance = loggerInstance.With(zap.Field{Key: "correlation-id", Type: zapcore.StringType, String: corrID})
		}
	})

	return loggerInstance, err
}

func NewOptionalLogger(correlationID string, debug bool) *zap.SugaredLogger {
	logger, err := NewLogger(correlationID, debug)
	if err != nil {
		return zap.NewNop().Sugar()
	}
	return logger
}
