package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	log "github.com/kyma-incubator/reconciler/pkg/reconciler/logger"
	"go.uber.org/zap"
)

type RemoteCallbackHandler struct {
	logger      *zap.SugaredLogger
	debug       bool
	callbackURL string
}

func NewRemoteCallbackHandler(callbackURL string, debug bool) (Handler, error) {
	//create logger
	logger, err := log.NewLogger()
	if err != nil {
		return nil, err
	}

	//validate URL
	if callbackURL != "" { //empty URLs are allowed (used in some test cases)
		if _, err := url.ParseRequestURI(callbackURL); err != nil {
			return nil, err
		}
	}

	//return new remote callback
	return &RemoteCallbackHandler{
		logger:      logger,
		debug:       debug,
		callbackURL: callbackURL,
	}, nil
}

func (cb *RemoteCallbackHandler) Callback(status reconciler.Status) error {
	if cb.callbackURL == "" { //test cases often don't provide a callback URL
		cb.logger.Warn("Empty callback-URL provided: remote callback not executed")
		return nil
	}

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
			cb.logger.Debugf("Response dump: %s", string(dumpResp))
		} else {
			cb.logger.Errorf("Failed to dump response: %s", dumpErr)
		}
	}

	if err != nil {
		cb.logger.Errorf("Status update request failed: %s", err)
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
