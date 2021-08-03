package callback

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	log "github.com/kyma-incubator/reconciler/pkg/reconciler/logger"
	"go.uber.org/zap"
)

type LocalCallbackHandler struct {
	logger      *zap.SugaredLogger
	callbackFct func(status reconciler.Status) error
}

func NewLocalCallbackHandler(callbackFct func(status reconciler.Status) error, debug bool) (Handler, error) {
	logger, err := log.NewLogger("", debug)
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
		cb.logger.Errorf("Calling local callback function failed: %s", err)
	}
	return err
}
