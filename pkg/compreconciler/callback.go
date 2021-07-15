package compreconciler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
	"net/http"
	"net/http/httputil"
)

type CallbackHandler interface {
	Callback(status Status) error
}

type defaultCallbackHandler struct {
	logger *zap.Logger
}

type CallbackHandlerFactory struct {
}

type RemoteCallbackHandler struct {
	*defaultCallbackHandler
	callbackURL string
}

func NewRemoteCallbackHandler(callbackURL string, debug bool) (CallbackHandler, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &RemoteCallbackHandler{
		&defaultCallbackHandler{
			logger: logger,
		},
		callbackURL,
	}, nil
}

func (cb *RemoteCallbackHandler) Callback(status Status) error {
	requestBody, err := json.Marshal(map[string]string{
		"status": string(status),
	})
	if err != nil {
		cb.logger.Error(err.Error())
	}

	resp, err := http.Post(cb.callbackURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		cb.logger.Error(fmt.Sprintf("Status update request failed: %s", err))
		//dump request
		dumpResp, err := httputil.DumpResponse(resp, true)
		if err == nil {
			cb.logger.Error(fmt.Sprintf("Failed to dump response: %s", err))
		} else {
			cb.logger.Info(fmt.Sprintf("Response is: %s", string(dumpResp)))
		}
	}

	return nil
}

type LocalCallbackHandler struct {
	*defaultCallbackHandler
	callbackFct func(status Status) error
}

func NewLocalCallbackHandler(callbackFct func(status Status) error, debug bool) (CallbackHandler, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &LocalCallbackHandler{
		&defaultCallbackHandler{
			logger: logger,
		},
		callbackFct,
	}, nil
}

func (cb *LocalCallbackHandler) Callback(status Status) error {
	err := cb.Callback(status)
	if err != nil {
		cb.logger.Info(fmt.Sprintf("Calling local callback function failed: %s", err))
	}
	return err
}
