package callback

import (
	"fmt"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
)

type LocalCallbackHandler struct {
	logger      *zap.Logger
	callbackFct func(status reconciler.Status) error
}

func NewLocalCallbackHandler(callbackFct func(status reconciler.Status) error, debug bool) (Handler, error) {
	logger, err := log.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &LocalCallbackHandler{
		logger:      logger,
		callbackFct: callbackFct,
	}, nil
}

func (cb *LocalCallbackHandler) Callback(status reconciler.Status) error {
	err := cb.callbackFct(status)
	if err != nil {
		cb.logger.Error(fmt.Sprintf("Calling local callback function failed: %s", err))
	}
	return err
}
