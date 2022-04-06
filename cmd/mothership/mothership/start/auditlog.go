package cmd

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"net/http"
)

const (
	XJWTHeaderName = "X-Jwt"
	// ExternalAddressHeaderName https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/headers#config-http-conn-man-headers-x-envoy-external-address
	ExternalAddressHeaderName = "X-Envoy-External-Address"
)

func NewLoggerWithFile(logFile string) (*zap.Logger, error) {
	cfg := zap.Config{
		Encoding:         "json",
		Level:            zap.NewAtomicLevelAt(zapcore.DebugLevel),
		OutputPaths:      []string{logFile},
		ErrorOutputPaths: []string{logFile},
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	ws := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    100, // megabytes
		MaxBackups: 5,
		MaxAge:     1,     // days
		Compress:   false, // save cpu cycles
	})
	// I need to replace the default core logger whit a new one that contains
	// WriterSyncer that wraps lumberjack. Lumberjack handels the log rotation.
	return logger.WithOptions(
		zap.WrapCore(func(zapcore.Core) zapcore.Core {
			return zapcore.NewCore(
				zapcore.NewJSONEncoder(zapcore.EncoderConfig{
					MessageKey:   "",
					LevelKey:     "level",
					EncodeLevel:  zapcore.CapitalLevelEncoder,
					TimeKey:      "time",
					EncodeTime:   zapcore.ISO8601TimeEncoder,
					EncodeCaller: zapcore.ShortCallerEncoder,
				}),
				ws,
				zap.InfoLevel,
			)
		}),
	), err
}

func newAuditLoggerMiddelware(l *zap.Logger, o *Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auditLogRequest(w, r, l, o)
			next.ServeHTTP(w, r)
		})
	}
}

func auditLogRequest(w http.ResponseWriter, r *http.Request, l *zap.Logger, o *Options) {
	//TODO Change or remove once Audit Log decision is made
}
