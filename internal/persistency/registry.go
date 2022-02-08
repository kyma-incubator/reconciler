package persistency

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/kv"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

type Registry struct {
	debug             bool
	logger            *zap.SugaredLogger
	connection        db.Connection
	inventory         cluster.Inventory
	kvRepository      *kv.Repository
	reconRepository   reconciliation.Repository
	occupancyRepo     occupancy.Repository
	occupancyTracking bool
	initialized       bool
}

func NewRegistry(cf db.ConnectionFactory, debug bool, occupancyTracking bool) (*Registry, error) {
	conn, err := cf.NewConnection()
	if err != nil {
		return nil, err
	}
	registry := &Registry{
		debug:             debug,
		connection:        conn,
		logger:            logger.NewLogger(debug),
		occupancyTracking: occupancyTracking,
	}
	return registry, registry.init()
}

func (or *Registry) init() error {
	if or.initialized {
		return nil
	}

	var err error
	if or.inventory, err = or.initInventory(); err != nil {
		return err
	}
	if or.kvRepository, err = or.initRepository(); err != nil {
		return err
	}
	if or.reconRepository, err = or.initReconciliationRepository(); err != nil {
		return err
	}
	if or.occupancyRepo, err = or.initOccupancyRepository(); err != nil {
		return err
	}

	or.initialized = true

	return nil
}

func (or *Registry) Close() error {
	if !or.initialized {
		return nil
	}
	return or.connection.Close()
}

func (or *Registry) Connection() db.Connection {
	return or.connection
}

func (or *Registry) Inventory() cluster.Inventory {
	return or.inventory
}

func (or *Registry) KVRepository() *kv.Repository {
	return or.kvRepository
}

func (or *Registry) ReconciliationRepository() reconciliation.Repository {
	return or.reconRepository
}

func (or *Registry) OccupancyRepository() occupancy.Repository {
	return or.occupancyRepo
}

func (or *Registry) initRepository() (*kv.Repository, error) {
	repository, err := kv.NewRepository(or.connection, or.debug)
	if err != nil {
		or.logger.Errorf("Failed to create configuration entry repository: %s", err)
	}
	return repository, err
}

func (or *Registry) initInventory() (cluster.Inventory, error) {
	collector := metrics.NewReconciliationStatusCollector()
	inventory, err := cluster.NewInventory(or.connection, or.debug, collector)
	if err != nil {
		or.logger.Errorf("Failed to create cluster inventory: %s", err)
	}
	return inventory, err
}

func (or *Registry) initReconciliationRepository() (reconciliation.Repository, error) {
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(or.connection, or.debug)
	if err != nil {
		or.logger.Errorf("Failed to create reconciliation repository: %s", err)
	}
	return reconRepo, err
}

func (or *Registry) initOccupancyRepository() (occupancy.Repository, error) {
	if !or.occupancyTracking {
		return &occupancy.MockRepository{
			CreateWorkerPoolOccupancyResult:   &model.WorkerPoolOccupancyEntity{},
			UpdateWorkerPoolOccupancyResult:   nil,
			RemoveWorkerPoolOccupancyResult:   nil,
			GetComponentListResult:            []string{"mothership"},
			GetWorkerPoolOccupanciesResult:    nil,
			FindWorkerPoolOccupancyByIDResult: &model.WorkerPoolOccupancyEntity{},
		}, nil
	}
	occupancyRepo, err := occupancy.NewPersistentOccupancyRepository(or.connection, or.debug)
	if err != nil {
		or.logger.Errorf("Failed to create occupancy repository: %s", err)
	}
	return occupancyRepo, err
}
