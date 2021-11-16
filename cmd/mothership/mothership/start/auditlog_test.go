package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	postKey       = "runtimeID"
	postValue     = "bb7fb804-ade5-42bc-a740-3c2861d0391d"
	tenantID      = "5f6b71a9-cd48-448d-9b58-9895f1639bc6"
	jwtPayloadSub = "test2@test.pl"
)

type MemorySink struct {
	*bytes.Buffer
}

func (s *MemorySink) Close() error { return nil }
func (s *MemorySink) Sync() error  { return nil }

func testLoggerWithOutput() (*zap.Logger, *MemorySink) {
	sink := &MemorySink{&bytes.Buffer{}}
	_ = zap.RegisterSink("memory", func(*url.URL) (zap.Sink, error) {
		return sink, nil
	})
	cfg := zap.Config{
		Encoding:         "json",
		Level:            zap.NewAtomicLevelAt(zapcore.DebugLevel),
		OutputPaths:      []string{"memory://"},
		ErrorOutputPaths: []string{"memory://"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:   "",
			LevelKey:     "level",
			EncodeLevel:  zapcore.CapitalLevelEncoder,
			TimeKey:      "time",
			EncodeTime:   zapcore.ISO8601TimeEncoder,
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}
	logger, _ := cfg.Build()

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
			name:   "delete request",
			method: http.MethodDelete,
		},
		{
			name:       "invalid jwtHeader",
			method:     http.MethodGet,
			expectFail: true,
			jwtHeader:  "eyJleHAiOjQ2ODU5ODk3MDAsImZvbyI6ImJhciIsImlhdCI6MTUzMjM4OTcwMCwiaXNzIjoidGVzdDJAdGVzdC5wbCIsInN1YiI6InRlc3QyQHRlc3QucGwifQ==",
		},
	}

	// build test logger
	logger, output := testLoggerWithOutput()
	// build reconciler options
	o := NewOptions(&cli.Options{})
	o.AuditLogTenantID = tenantID

	for _, testCase := range testCases {
		// GIVEN
		w := httptest.NewRecorder()

		req, _ := http.NewRequest(testCase.method, "http://localhost/v1/clusters", nil)

		req = mux.SetURLVars(req, map[string]string{
			paramContractVersion: "1",
		})

		if testCase.method == http.MethodPost {
			req.Body = io.NopCloser(bytes.NewBuffer([]byte(testCase.body)))
		}
		if testCase.jwtHeader != "" {
			req.Header.Add(XJWTHeaderName, testCase.jwtHeader)
		}

		// log and test
		auditLogRequest(w, req, logger, o)
		if testCase.expectFail {
			assert.Equalf(t, w.Result().StatusCode,
				http.StatusInternalServerError,
				"expected http status: %v, got: %v",
				http.StatusInternalServerError,
				w.Result().StatusCode)
			continue
		}

		t.Log(output.String())
		err := validateLog(output.String(), testCase.method, testCase.jwtHeader != "")
		assert.NoError(t, err)

		// clean the log sink
		output.Reset()
	}
}

// validateLog ensures that all required fields in the log message are set and valid. If any of these is missing the audit log backend will not accept/process our logs
func validateLog(logMsg, method string, useJWT bool) error {
	l := &log{}
	if err := json.Unmarshal([]byte(logMsg), l); err != nil {
		return err
	}
	if l.Time == "" ||
		l.UUID == "" ||
		l.User == "" ||
		l.Data == "" ||
		l.Tenant == "" ||
		l.IP == "" ||
		l.Category == "" {
		return errors.New(fmt.Sprintf("empty log field: %#v", l))
	}
	if l.Tenant != tenantID {
		return errors.New(fmt.Sprintf("invalid log tenantID: expected: %s, got: %s", tenantID, l.Tenant))
	}
	if useJWT && l.User != jwtPayloadSub {
		return errors.New(fmt.Sprintf("invalid user: expected: %s, got: %s", jwtPayloadSub, l.User))

	}
	if method == http.MethodPost {
		d := &data{}
		if err := json.Unmarshal([]byte(l.Data), d); err != nil {
			return err
		}
		if d.RequestBody == "" {
			return errors.New(fmt.Sprintf("empty request body in log message data field: %#v", l.Data))
		}
	}
	return nil
}
