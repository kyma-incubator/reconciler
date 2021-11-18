package internal

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggingWarningHandler(t *testing.T) {
	t.Run("should log if code is 299", func(t *testing.T) {
		core, recorded := observer.New(zapcore.WarnLevel)
		sut := loggingWarningHandler{logger: zap.New(core).Sugar()}
		sut.HandleWarningHeader(299, "", "foo")
		logs := recorded.All()
		require.Len(t, logs, 1)
		require.Equal(t, "foo", logs[0].Message)
	})

	t.Run("should not log if code is not 299", func(t *testing.T) {
		core, recorded := observer.New(zapcore.WarnLevel)
		sut := loggingWarningHandler{logger: zap.New(core).Sugar()}
		sut.HandleWarningHeader(http.StatusOK, "", "foo")
		logs := recorded.All()
		require.Len(t, logs, 0)
	})
}
