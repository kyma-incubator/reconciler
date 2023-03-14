package invoker

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/internal/cli/test"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestRemoteInvoker(t *testing.T) {
	reconRepo := reconciliation.NewInMemoryReconciliationRepository()

	//create reconciliation entity
	reconEntity, err := reconRepo.CreateReconciliation(clusterStateMock, &model.ReconciliationSequenceConfig{})
	require.NoError(t, err)

	//retrieve ops of reconciliation entity
	opEntities, err := reconRepo.GetOperations(&operation.WithSchedulingID{
		SchedulingID: reconEntity.SchedulingID,
	})
	require.NoError(t, err)
	if clusterStateMock.Status.Status.IsDeletionInProgress() {
		require.Len(t, opEntities, 7, "reconciliation sequence has 7 ops (5 + crds + cleaner)")
	} else {
		require.Len(t, opEntities, 6, "reconciliation sequence has 6 ops (5 + crds)")
	}

	ctx, cancel := context.WithCancel(context.Background())
	startServer(ctx, t)
	defer shotdownServer(cancel, t)

	t.Run("Invoke without base reconciler", func(t *testing.T) {
		cfg := &config.Config{
			Scheme: "https",
			Host:   "mothership-reconciler",
			Port:   443,
			Scheduler: config.SchedulerConfig{
				PreComponents: nil,
				Reconcilers:   nil,
			},
		}
		err := invokeRemoteInvoker(reconRepo, opEntities[0], cfg)
		require.Error(t, err)
		require.True(t, IsNoFallbackReconcilerDefinedError(err))
	})

	t.Run("Invoke non component-reconciler (invalid URL)", func(t *testing.T) {
		cfg := &config.Config{
			Scheme: "https",
			Host:   "mothership-reconciler",
			Port:   443,
			Scheduler: config.SchedulerConfig{
				PreComponents: nil,
				Reconcilers: map[string]config.ComponentReconciler{
					"base": {
						URL: "https://idontexist.url/post",
					},
				},
			},
		}
		err := invokeRemoteInvoker(reconRepo, opEntities[1], cfg)
		require.Error(t, err)

		//invocation of component reconciler failed... marking operation to be in error state
		requireOperationState(t, reconRepo, opEntities[1], model.OperationStateError)
	})

	t.Run("Invoke component-reconciler: happy path", func(t *testing.T) {
		cfg := &config.Config{
			Scheme: "https",
			Host:   "mothership-reconciler",
			Port:   443,
			Scheduler: config.SchedulerConfig{
				PreComponents: nil,
				Reconcilers: map[string]config.ComponentReconciler{
					"base": {
						URL: "http://127.0.0.1:5555/200",
					},
				},
			},
		}
		err := invokeRemoteInvoker(reconRepo, opEntities[2], cfg)
		require.NoError(t, err)

		requireOperationState(t, reconRepo, opEntities[2], model.OperationStateInProgress)
	})

	t.Run("Invoke component-reconciler: return 400 error", func(t *testing.T) {
		cfg := &config.Config{
			Scheme: "https",
			Host:   "mothership-reconciler",
			Port:   443,
			Scheduler: config.SchedulerConfig{
				PreComponents: nil,
				Reconcilers: map[string]config.ComponentReconciler{
					"base": {
						URL: "http://127.0.0.1:5555/400",
					},
				},
			},
		}
		err := invokeRemoteInvoker(reconRepo, opEntities[3], cfg)
		require.NoError(t, err)

		requireOperationState(t, reconRepo, opEntities[3], model.OperationStateClientError)
	})

	t.Run("Invoke component-reconciler: return 500 error with error JSON response", func(t *testing.T) {
		cfg := &config.Config{
			Scheme: "https",
			Host:   "mothership-reconciler",
			Port:   443,
			Scheduler: config.SchedulerConfig{
				PreComponents: nil,
				Reconcilers: map[string]config.ComponentReconciler{
					"base": {
						URL: "http://127.0.0.1:5555/500nice",
					},
				},
			},
		}
		err := invokeRemoteInvoker(reconRepo, opEntities[4], cfg)
		require.NoError(t, err)

		requireOperationState(t, reconRepo, opEntities[4], model.OperationStateClientError)
	})

	t.Run("Invoke component-reconciler: return 500 error with invalid error response", func(t *testing.T) {
		cfg := &config.Config{
			Scheme: "https",
			Host:   "mothership-reconciler",
			Port:   443,
			Scheduler: config.SchedulerConfig{
				PreComponents: nil,
				Reconcilers: map[string]config.ComponentReconciler{
					"base": {
						URL: "http://127.0.0.1:5555/500bad",
					},
				},
			},
		}
		err := invokeRemoteInvoker(reconRepo, opEntities[5], cfg)
		require.NoError(t, err)

		requireOperationState(t, reconRepo, opEntities[5], model.OperationStateClientError)
	})
}

func invokeRemoteInvoker(reconRepo reconciliation.Repository, op *model.OperationEntity, cfg *config.Config) error {
	//reset operation state
	if err := reconRepo.UpdateOperationState(op.SchedulingID, op.CorrelationID, model.OperationStateNew, false); err != nil {
		if !reconciliation.IsAlreadyInStateError(err) {
			return err
		}
	}

	invoker := NewRemoteReconcilerInvoker(reconRepo, cfg, logger.NewLogger(true))
	return invoker.Invoke(context.Background(), &Params{
		ComponentToReconcile: &keb.Component{
			Component: model.CRDComponent,
			Version:   "1.2.3",
		},
		ComponentsReady: nil,
		ClusterState:    clusterStateMock,
		SchedulingID:    op.SchedulingID,
		CorrelationID:   op.CorrelationID,
	})
}

func shotdownServer(cancel context.CancelFunc, t *testing.T) {
	cancel()
	test.WaitForFreeTCPSocket(t, "127.0.0.1", 5555, 5*time.Second)
}

func startServer(ctx context.Context, t *testing.T) {
	go func() {
		//react on provided URL
		router := mux.NewRouter()
		router.HandleFunc(
			"/200",
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "application/json")
				if err := json.NewEncoder(w).Encode(&reconciler.HTTPReconciliationResponse{}); err != nil {
					server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
						Error: errors.Wrap(err, "failed to encode response payload to JSON").Error(),
					})
				}
			}).
			Methods("PUT", "POST")

		router.HandleFunc(
			"/400",
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "application/json")
				server.SendHTTPError(w, http.StatusNotFound, &reconciler.HTTPErrorResponse{
					Error: "the thing you are looking for could not be found",
				})
			}).
			Methods("PUT", "POST")

		router.HandleFunc(
			"/500nice",
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "application/json")
				server.SendHTTPError(w, http.StatusInternalServerError, &reconciler.HTTPErrorResponse{
					Error: "Simulating a controlled failure situation in component reconciler",
				})
			}).
			Methods("PUT", "POST")

		router.HandleFunc(
			"/500bad",
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "application/json")
				server.SendHTTPError(w, http.StatusInternalServerError, "This is not a JSON")
			}).
			Methods("PUT", "POST")

		//start server
		err := (&server.Webserver{
			Logger: logger.NewLogger(true),
			Port:   5555,
			Router: router,
		}).Start(ctx)
		require.NoError(t, err)
	}()
	test.WaitForTCPSocket(t, "127.0.0.1", 5555, 5*time.Second)
}
