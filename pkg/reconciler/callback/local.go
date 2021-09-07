package callback

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
)

type LocalCallbackHandler struct {
	logger       *zap.SugaredLogger
	callbackFunc func(msg *reconciler.CallbackMessage) error
}

func NewLocalCallbackHandler(callbackFunc func(msg *reconciler.CallbackMessage) error, logger *zap.SugaredLogger) (Handler, error) {
	return &LocalCallbackHandler{
		logger:       logger,
		callbackFunc: callbackFunc,
	}, nil
}

func (cb *LocalCallbackHandler) Callback(msg *reconciler.CallbackMessage) error {
	err := cb.callbackFunc(msg)
	if err != nil {
		cb.logger.Errorf("Calling local callback function failed: %s", err)
	}
	return err
}
