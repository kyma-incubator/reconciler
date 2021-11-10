package internal

import (
	"go.uber.org/zap"
)

type loggingWarningHandler struct {
	logger *zap.SugaredLogger
}

//HandleWarningHeader logs code 299 warnings received from the API server
//similar to the default warning header https://github.com/kubernetes/client-go/blob/master/rest/warnings.go#L64
func (lwh *loggingWarningHandler) HandleWarningHeader(code int, agent string, text string) {
	if lwh.logger == nil || code != 299 || len(text) == 0 {
		return
	}

	lwh.logger.Warn(text)
}
