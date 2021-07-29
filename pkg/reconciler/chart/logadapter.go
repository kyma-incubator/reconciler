package chart

import (
	"fmt"

	"go.uber.org/zap"
)

//NewHydroformLoggerAdapter adapts a ZAP logger to a Hydrofrom compatible logger
func NewHydroformLoggerAdapter(logger *zap.Logger) *HydroformLoggerAdapter {
	return &HydroformLoggerAdapter{
		logger: logger,
	}
}

//HydroformLoggerAdapter is implementing the logger interface of Hydroform
//to make it compatible with the ZAP logger API.
type HydroformLoggerAdapter struct {
	logger *zap.Logger
}

func (l *HydroformLoggerAdapter) Info(args ...interface{}) {
	l.logger.Info(fmt.Sprintf("%v", args))
}
func (l *HydroformLoggerAdapter) Infof(template string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(template, args...))
}
func (l *HydroformLoggerAdapter) Warn(args ...interface{}) {
	l.logger.Warn(fmt.Sprintf("%v", args...))
}
func (l *HydroformLoggerAdapter) Warnf(template string, args ...interface{}) {
	l.logger.Warn(fmt.Sprintf(template, args...))
}
func (l *HydroformLoggerAdapter) Error(args ...interface{}) {
	l.logger.Error(fmt.Sprintf("%v", args))
}
func (l *HydroformLoggerAdapter) Errorf(template string, args ...interface{}) {
	l.logger.Error(fmt.Sprintf(template, args...))
}
func (l *HydroformLoggerAdapter) Fatal(args ...interface{}) {
	l.logger.Fatal(fmt.Sprintf("%v", args))
}
func (l *HydroformLoggerAdapter) Fatalf(template string, args ...interface{}) {
	l.logger.Fatal(fmt.Sprintf(template, args...))
}
