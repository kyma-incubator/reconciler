package app

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/kv"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/metrics"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

type ApplicationRegistry struct {
	debug             bool
	logger            *zap.SugaredLogger
	connectionFactory db.ConnectionFactory
	connection        db.Connection
	inventory         cluster.Inventory
	kvRepository      *kv.Repository
	reconRepository   reconciliation.Repository
	initialized       bool
}

func NewApplicationRegistry(cf db.ConnectionFactory, debug bool) (*ApplicationRegistry, error) {
	conn, err := cf.NewConnection()
	if err != nil {
		return nil, err
	}
	registry := &ApplicationRegistry{
		debug:             debug,
		connectionFactory: cf,
		connection:        conn,
		logger:            logger.NewLogger(debug),
	}
	return registry, registry.init()
}

func (or *ApplicationRegistry) init() error {
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

	or.initialized = true

	return nil
}

func (or *ApplicationRegistry) Close() error {
	if !or.initialized {
		return nil
	}
	return or.connection.Close()
}

func (or *ApplicationRegistry) Connnection() db.Connection {
	return or.connection
}

func (or *ApplicationRegistry) Inventory() cluster.Inventory {
	return or.inventory
}

func (or *ApplicationRegistry) KVRepository() *kv.Repository {
	return or.kvRepository
}

func (or *ApplicationRegistry) ReconciliationRepository() reconciliation.Repository {
	return or.reconRepository
}

func (or *ApplicationRegistry) initRepository() (*kv.Repository, error) {
	repository, err := kv.NewRepository(or.connection, or.debug)
	if err != nil {
		or.logger.Errorf("Failed to create configuration entry repository: %s", err)
	}
	return repository, err
}

func (or *ApplicationRegistry) initInventory() (cluster.Inventory, error) {
	collector := metrics.NewReconciliationStatusCollector()
	inventory, err := cluster.NewInventory(or.connection, or.debug, collector)
	if err != nil {
		or.logger.Errorf("Failed to create cluster inventory: %s", err)
	}
	return inventory, err
}

func (or *ApplicationRegistry) initReconciliationRepository() (reconciliation.Repository, error) {
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(or.connection, or.debug)
	if err != nil {
		or.logger.Errorf("Failed to create reconciliation repository: %s", err)
	}
	return reconRepo, err
}
