package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"os"
	"testing"
)

const (
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatPlain OutputFormat = "plain"
)

var outputFormat = OutputFormatJSON

type OutputFormat string

func NewLogger(debug bool) *zap.SugaredLogger {
	logLevel := zapcore.InfoLevel
	if debug {
		logLevel = zapcore.DebugLevel
	}
	return newLogger(logLevel).Sugar()
}

func NewTestLogger(t *testing.T) *zap.SugaredLogger {
	return zaptest.NewLogger(t).Sugar()
}

func newLogger(logLevel zapcore.Level) *zap.Logger {
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:   "message",
		LevelKey:     "level",
		EncodeLevel:  zapcore.CapitalLevelEncoder,
		TimeKey:      "time",
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		CallerKey:    "caller",
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	switch outputFormat {
	case OutputFormatPlain:
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	case OutputFormatJSON:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	return zap.New(
		zapcore.NewCore(
			encoder,
			zapcore.Lock(os.Stderr),
			zap.NewAtomicLevelAt(logLevel),
		),
		zap.ErrorOutput(os.Stderr))
}

func SetOutputFormat(of OutputFormat) {
	outputFormat = of
}
