package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/internal/cli"
	cliRecon "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/kyma-incubator/reconciler/internal/persistency"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	eventLog "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"testing"
	"time"
)

type ComponentReconcilerBootstrap struct {
	Name      string
	Namespace string
	Version   string
	URL       string

	WorkerConfig     *cliRecon.WorkerConfig
	ReconcilerAction service.Action

	DeleteAfterTest bool
}

func OptionsForComponentReconciler(
	t *testing.T,
	registry *persistency.Registry,
) *cliRecon.Options {
	t.Helper()
	s := require.New(t)
	cliOptions := &cli.Options{Verbose: false}
	var err error
	cliOptions.Registry = registry
	s.NoError(err)
	o := cliRecon.NewOptions(cliOptions)
	o.HeartbeatSenderConfig.Interval = 2 * time.Second
	o.ProgressTrackerConfig.Interval = 3 * time.Second
	o.RetryConfig.RetryDelay = 2 * time.Second
	o.RetryConfig.MaxRetries = 3
	return o
}

type NoOpReconcileAction struct {
	WaitTime time.Duration
}

// Run reconciler Action logic for Eventing. It executes the Action steps in order
// and returns a non-nil error if any step was unsuccessful.
func (a *NoOpReconcileAction) Run(context *service.ActionContext) (err error) {
	// prepare logger
	contextLogger := eventLog.ContextLogger(context, eventLog.WithAction("no-op-action"))
	contextLogger.Infof("Waiting to simulate Op...")
	time.Sleep(a.WaitTime)
	return nil
}

type MockComponentReconcilerHandler interface {
	Handle(ctx context.Context, t *testing.T, params *server.Params, writer http.ResponseWriter, model *reconciler.Task)
}
type MockComponentReconcilerRouter struct {
	ctx context.Context
	t   *testing.T
	*mux.Router
	runHandler []MockComponentReconcilerHandler
}

func StartMockComponentReconciler(ctx context.Context, t *testing.T, options *cliRecon.Options, handlers ...MockComponentReconcilerHandler) {
	a := require.New(t)
	r := &MockComponentReconcilerRouter{ctx, t, mux.NewRouter(), handlers}
	r.Router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	})
	r.Router.HandleFunc(fmt.Sprintf("/v{%s}/run", "version"), func(writer http.ResponseWriter, request *http.Request) {
		params := server.NewParams(request)
		model := &reconciler.Task{}
		b, err := io.ReadAll(request.Body)
		a.NoError(err)
		a.NoError(json.Unmarshal(b, model))
		writer.Header().Set("content-type", "application/json")
		for _, handler := range r.runHandler {
			handler.Handle(ctx, t, params, writer, model)
		}
	}).Methods("PUT", "POST")

	s := &server.Webserver{
		Logger:     options.Logger(),
		Port:       options.ServerConfig.Port,
		SSLCrtFile: options.ServerConfig.SSLCrtFile,
		SSLKeyFile: options.ServerConfig.SSLKeyFile,
		Router:     r.Router,
	}
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		t.Cleanup(func() {
			cancel()
			time.Sleep(1 * time.Second) //Allow graceful shutdown
		})
		require.NoError(t, s.Start(ctx))
	}()
}
