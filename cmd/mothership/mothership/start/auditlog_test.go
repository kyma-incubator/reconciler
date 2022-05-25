package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	postKey            = "runtimeID"
	postValue          = "bb7fb804-ade5-42bc-a740-3c2861d0391d"
	postSanitizedKey   = "kubeconfig"
	postSanitizedValue = "SOME DATA TO REDACT"
	tenantID           = "5f6b71a9-cd48-448d-9b58-9895f1639bc6"
	jwtPayloadSub      = "test2@test.pl"
)

type MemorySink struct {
	*bytes.Buffer
}

func (s *MemorySink) Close() error { return nil }
func (s *MemorySink) Sync() error  { return nil }

func testLoggerWithOutput(t *testing.T) (*zap.Logger, *MemorySink) {
	sink := &MemorySink{&bytes.Buffer{}}
	err := zap.RegisterSink("memory", func(*url.URL) (zap.Sink, error) {
		return sink, nil
	})
	require.NoError(t, err)
	cfg := zap.Config{
		Encoding:         "json",
		Level:            zap.NewAtomicLevelAt(zapcore.DebugLevel),
		OutputPaths:      []string{"memory://"},
		ErrorOutputPaths: []string{"memory://"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:  "data",
			LevelKey:    "level",
			EncodeLevel: zapcore.CapitalLevelEncoder,
			TimeKey:     "time",
			EncodeTime: func(t time.Time, pae zapcore.PrimitiveArrayEncoder) {
				zapcore.RFC3339TimeEncoder(t.UTC(), pae)
			},
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}
	logger, err := cfg.Build()
	require.NoError(t, err)

	return logger, sink
}

type log struct {
	Time     string `json:"time"`
	UUID     string `json:"uuid"`
	User     string `json:"user"`
	Data     string `json:"data"`
	Tenant   string `json:"tenant"`
	IP       string `json:"ip"`
	Category string `json:"category"`
}

func Test_Auditlog_With_Middleware(t *testing.T) {
	// build reconciler options
	o := NewOptions(&cli.Options{})
	o.AuditLogTenantID = tenantID
	a := require.New(t)
	auditLog := filepath.Join(t.TempDir(), "auditlog")
	l, err := NewLoggerWithFile(auditLog)
	a.NoError(err)
	w := httptest.NewRecorder()
	mw := newAuditLoggerMiddleware(l, o)

	req, _ := http.NewRequest(http.MethodPost, "http://localhost/v1/clusters", nil)

	req = mux.SetURLVars(req, map[string]string{
		paramContractVersion: "1",
	})

	req.Body = io.NopCloser(bytes.NewBuffer([]byte(fmt.Sprintf(`{"%s":"%s", "%s":"%s"}`, postKey, postValue, postSanitizedKey, postSanitizedValue))))

	mw(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {})).ServeHTTP(w, req)

	auditLogFile, auditLogFileErr := os.Open(auditLog)
	a.NoError(auditLogFileErr)
	scanner := bufio.NewScanner(auditLogFile)

	for scanner.Scan() {
		logOutput := make(map[string]interface{})
		a.NoError(json.Unmarshal(scanner.Bytes(), &logOutput))
		a.NotNil(logOutput["time"])
		a.NotNil(logOutput["level"])
		a.NotNil(logOutput["uuid"])
		a.NotNil(logOutput["user"])
		a.NotNil(logOutput["tenant"])
		a.NotNil(logOutput["category"])
		a.NotNil(logOutput["time"])
		a.Nil(logOutput["ip"])

		a.NotNil(logOutput[DataMessageKey])
		data, requestBody := data{}, make(map[string]interface{})
		a.NoError(json.Unmarshal([]byte(logOutput[DataMessageKey].(string)), &data))
		a.NoError(json.Unmarshal([]byte(data.RequestBody), &requestBody))
		a.Equal(redacted, requestBody[postSanitizedKey])
	}
}

func Test_Auditlog(t *testing.T) {
	testCases := []struct {
		name       string
		method     string
		body       string
		jwtHeader  string
		expectFail bool
	}{
		{
			name:   "get request",
			method: http.MethodGet,
		},
		{
			name:      "get request with jwtHeader",
			method:    http.MethodGet,
			jwtHeader: "eyJleHAiOjQ2ODU5ODk3MDAsImZvbyI6ImJhciIsImlhdCI6MTUzMjM4OTcwMCwiaXNzIjoidGVzdDJAdGVzdC5wbCIsInN1YiI6InRlc3QyQHRlc3QucGwifQ",
		},
		{
			name:   "post request",
			method: http.MethodPost,
			body:   fmt.Sprintf(`{"%s":"%s"}`, postKey, postValue),
		},
		{
			name:   "post request",
			method: http.MethodPost,
			body:   fmt.Sprintf(`{"%s":"%s"}`, postKey, postValue),
		},
		{
			name:   "delete request",
			method: http.MethodDelete,
		},
		{
			name:       "invalid jwtHeader",
			method:     http.MethodPost,
			expectFail: true,
			jwtHeader:  "eyJleHAiOjQ2ODU5ODk3MDAsImZvbyI6ImJhciIsImlhdCI6MTUzMjM4OTcwMCwiaXNzIjoidGVzdDJAdGVzdC5wbCIsInN1YiI6InRlc3QyQHRlc3QucGwifQ==",
		},
	}

	// build test logger
	logger, output := testLoggerWithOutput(t)
	// build reconciler options
	o := NewOptions(&cli.Options{})
	o.AuditLogTenantID = tenantID

	for _, testCase := range testCases {
		// GIVEN
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			req, _ := http.NewRequest(tc.method, "http://localhost/v1/clusters", nil)

			req = mux.SetURLVars(req, map[string]string{
				paramContractVersion: "1",
			})

			if tc.method == http.MethodPost {
				req.Body = io.NopCloser(bytes.NewBuffer([]byte(tc.body)))
			}

			if tc.jwtHeader != "" {
				req.Header.Add(XJWTHeaderName, tc.jwtHeader)
			}

			// clean the log sink
			defer output.Reset()
			// WHEN
			auditLogRequest(w, req, logger, o)

			// THEN
			if tc.expectFail {
				require.Equalf(t, http.StatusInternalServerError, w.Result().StatusCode,
					"expected http status: %v, got: %v",
					http.StatusInternalServerError, w.Result().StatusCode)
			} else {
				t.Log(output.String())
				validateLog(t, output.String(), tc.method, tc.jwtHeader != "")
			}

		})
	}
}

// validateLog ensures that all required fields in the log message are set and valid. If any of these is missing the audit log backend will not accept/process our logs
func validateLog(t *testing.T, logMsg, method string, useJWT bool) {
	l := &log{}
	err := json.Unmarshal([]byte(logMsg), l)
	require.NoError(t, err)

	require.Falsef(t, l.Time == "" ||
		l.UUID == "" ||
		l.User == "" ||
		l.Data == "" ||
		l.Tenant == "" ||
		l.Category == "", "empty log field: %#v", l)

	require.Equalf(t, tenantID, l.Tenant, "invalid log tenantID: expected: %s, got: %s", tenantID, l.Tenant)

	if useJWT {
		require.Equalf(t, jwtPayloadSub, l.User, "invalid user: expected: %s, got: %s", jwtPayloadSub, l.User)
	}

	if method == http.MethodPost {
		d := &data{}
		err := json.Unmarshal([]byte(l.Data), d)
		require.NoError(t, err)
		require.NotEmptyf(t, d.RequestBody, "empty request body in log message data field: %#v", l.Data)
	}
}
