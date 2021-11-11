package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/server"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	XJWTHeaderName = "X-Jwt"
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
		MaxAge:     14,    // days
		Compress:   false, // save cpu cycles
	})
	// I need to replace the default core logger whit a new one that contains
	// WriterSyncer that wraps luberjack. Lumberjack handels the log rotation.
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

func NewAuditLoggerMiddelware(l *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auditLogRequest(w, r, l)

			next.ServeHTTP(w, r)
		})
	}
}

func auditLogRequest(w http.ResponseWriter, r *http.Request, l *zap.Logger) {
	params := server.NewParams(r)
	contractV, err := params.Int64(paramContractVersion)
	if err != nil {

		server.SendHTTPError(w, http.StatusBadRequest, &keb.HTTPErrorResponse{
			Error: errors.Wrap(err, "Contract version undefined").Error(),
		})
		return
	}
	// log basic request information
	l = l.With(zap.String("contractVersion", fmt.Sprint(contractV))).
		With(zap.String("method", r.Method)).
		With(zap.String("URI", r.RequestURI))

	// log auth/authn information if available
	if jwtHeader := r.Header.Get(XJWTHeaderName); len(jwtHeader) != 0 {
		decodedSeg, err := base64.RawURLEncoding.DecodeString(jwtHeader)
		if err != nil {
			server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
				Error: errors.Wrap(err, fmt.Sprintf("Failed to parse %s header content ", XJWTHeaderName)).Error(),
			})
			return
		}

		l = l.With(zap.String(XJWTHeaderName, string(decodedSeg)))
	}

	// log request body if needed.
	if r.Method == "POST" || r.Method == "PUT" {
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			server.SendHTTPError(w, http.StatusInternalServerError, &keb.HTTPErrorResponse{
				Error: errors.Wrap(err, "Failed to read received JSON payload").Error(),
			})
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))
		l = l.With(zap.String("requestBody", string(reqBody)))
	}
	// log
	l.Info("")
}
