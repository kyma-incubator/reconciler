package invoker

//
//type remoteReconcilerInvoker struct {
//	logger           *zap.SugaredLogger
//	mothershipScheme string
//	mothershipHost   string
//	mothershipPort   int
//}
//
//func (rri *remoteReconcilerInvoker) Invoke(ctx context.Context, params *Params) error {
//	payload := params.CreateRemoteReconciliation(fmt.Sprintf("%s://%s:%d/v1/operations/%s/callback/%s",
//		rri.mothershipScheme, rri.mothershipHost, rri.mothershipPort, params.SchedulingID, params.CorrelationID))
//
//	jsonPayload, err := json.Marshal(payload)
//	if err != nil {
//		return fmt.Errorf("failed to marshal payload for reconciler call: %s", err)
//	}
//
//	rri.logger.Debugf("Calling the reconciler for a component %s, correlation ID: %s", payload.Component, params.CorrelationID)
//	resp, err := http.Post(params.ReconcilerURL, "application/json", bytes.NewBuffer(jsonPayload))
//	if err != nil {
//		return fmt.Errorf("failed to call reconciler: %s", err)
//	}
//	defer func() {
//		if err := resp.Body.Close(); err != nil {
//			rri.logger.Errorf("Error while closing the response body: %s", err)
//		}
//	}()
//
//	body, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		return fmt.Errorf("failed to read the response body: %s", err)
//	}
//	rri.logger.Debugf("Called the reconciler for a component %s, correlation ID: %s, got status %s", payload.Component, params.CorrelationID, resp.Status)
//	_ = body // TODO: handle the reconciler response body
//
//	if resp.StatusCode != http.StatusOK {
//		if resp.StatusCode == http.StatusPreconditionRequired {
//			return fmt.Errorf("failed preconditions: %s", resp.Status)
//		}
//		return fmt.Errorf("reconciler responded with status: %s", resp.Status)
//	}
//	// At this point we can assume that the call was successful
//	// and the component reconciler is doing the job of reconciliation
//	return nil
//}
