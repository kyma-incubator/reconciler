package chart

import (
	"go.uber.org/zap"
)

//NewHydroformLoggerAdapter adapts a ZAP logger to a Hydrofrom compatible logger
func NewHydroformLoggerAdapter(logger *zap.SugaredLogger) *HydroformLoggerAdapter {
	return &HydroformLoggerAdapter{
		logger: logger,
	}
}

//HydroformLoggerAdapter is implementing the logger interface of Hydroform
//to make it compatible with the ZAP logger API.
type HydroformLoggerAdapter struct {
	logger *zap.SugaredLogger
}

func (l *HydroformLoggerAdapter) Info(args ...interface{}) {
	l.logger.Info(args)
}
func (l *HydroformLoggerAdapter) Infof(template string, args ...interface{}) {
	l.logger.Info(template, args)
}
func (l *HydroformLoggerAdapter) Warn(args ...interface{}) {
	l.logger.Warn(args)
}
func (l *HydroformLoggerAdapter) Warnf(template string, args ...interface{}) {
	l.logger.Warn(template, args)
}
func (l *HydroformLoggerAdapter) Error(args ...interface{}) {
	l.logger.Error(args)
}
func (l *HydroformLoggerAdapter) Errorf(template string, args ...interface{}) {
	l.logger.Error(template, args)
}
func (l *HydroformLoggerAdapter) Fatal(args ...interface{}) {
	l.logger.Fatal(args)
}
func (l *HydroformLoggerAdapter) Fatalf(template string, args ...interface{}) {
	l.logger.Fatal(template, args)
}
