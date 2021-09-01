package callback

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
)

type LocalCallbackHandler struct {
	logger       *zap.SugaredLogger
	callbackFunc func(status reconciler.Status) error
}

func NewLocalCallbackHandler(callbackFunc func(status reconciler.Status) error, logger *zap.SugaredLogger) (Handler, error) {
	return &LocalCallbackHandler{
		logger:       logger,
		callbackFunc: callbackFunc,
	}, nil
}

func (cb *LocalCallbackHandler) Callback(status reconciler.Status) error {
	err := cb.callbackFunc(status)
	if err != nil {
		cb.logger.Errorf("Calling local callback function failed: %s", err)
	}
	return err
}
