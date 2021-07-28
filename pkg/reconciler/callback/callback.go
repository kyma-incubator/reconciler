package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type CallbackHandler interface {
	Callback(status reconciler.Status) error
}

type RemoteCallbackHandler struct {
	logger      *zap.Logger
	debug       bool
	callbackURL string
}

func NewRemoteCallbackHandler(callbackURL string, debug bool) (CallbackHandler, error) {
	//create logger
	logger, err := log.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	//validate URL
	if _, err := url.ParseRequestURI(callbackURL); err != nil {
		return nil, err
	}
	//return new remote callback
	return &RemoteCallbackHandler{
		logger:      logger,
		debug:       debug,
		callbackURL: callbackURL,
	}, nil
}

func (cb *RemoteCallbackHandler) Callback(status reconciler.Status) error {
	requestBody, err := json.Marshal(map[string]string{
		"status": string(status),
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(cb.callbackURL, "application/json", bytes.NewBuffer(requestBody))

	//dump request
	if cb.debug {
		dumpResp, dumpErr := httputil.DumpResponse(resp, true)
		if err == nil {
			cb.logger.Debug(fmt.Sprintf("Response dump: %s", string(dumpResp)))
		} else {
			cb.logger.Error(fmt.Sprintf("Failed to dump response: %s", dumpErr))
		}
	}

	if err != nil {
		cb.logger.Error(fmt.Sprintf("Status update request failed: %s", err))
		return err
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("Status update request (status '%s')  failed with '%d' HTTP response code",
			status,
			resp.StatusCode)
		cb.logger.Info(msg)
		return fmt.Errorf(msg)
	}

	return nil
}

type LocalCallbackHandler struct {
	logger      *zap.Logger
	callbackFct func(status reconciler.Status) error
}

func NewLocalCallbackHandler(callbackFct func(status reconciler.Status) error, debug bool) (CallbackHandler, error) {
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
