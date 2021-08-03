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
)

func InitLogger(correlationID string, debug bool) (*zap.SugaredLogger, error) {
	if correlationID == "" {
		return nil, fmt.Errorf("Correlation ID is empty. Logger cannot be created without the correlation ID.")
	}

	var err error
	once.Do(func() { // atomic, does not allow repeating
		loggerInstance, err = logger.NewLogger(debug)
		if loggerInstance != nil {
			loggerInstance = loggerInstance.With(zap.Field{Key: "correlation-id", Type: zapcore.StringType, String: correlationID})
		}
	})

	return loggerInstance, err
}

func NewLogger() (*zap.SugaredLogger, error) {
	if loggerInstance == nil {
		return nil, fmt.Errorf("InitLogger must be called to initialize the logger.")
	}
	return loggerInstance, nil
}

func NewOptionalLogger() *zap.SugaredLogger {
	logger, err := NewLogger()
	if err != nil {
		return zap.NewNop().Sugar()
	}
	return logger
}
