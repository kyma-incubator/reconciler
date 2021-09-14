package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

func NewLogger(debug bool) *zap.SugaredLogger {
	logLevel := zapcore.InfoLevel
	if debug {
		logLevel = zapcore.DebugLevel
	}

	logger := zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(zapcore.EncoderConfig{
				MessageKey:   "message",
				LevelKey:     "level",
				EncodeLevel:  zapcore.CapitalLevelEncoder,
				TimeKey:      "time",
				EncodeTime:   zapcore.ISO8601TimeEncoder,
				CallerKey:    "caller",
				EncodeCaller: zapcore.ShortCallerEncoder,
			}),
			zapcore.Lock(os.Stdout),
			zap.NewAtomicLevelAt(logLevel),
		),
		zap.ErrorOutput(os.Stderr))

	return logger.Sugar()
}
