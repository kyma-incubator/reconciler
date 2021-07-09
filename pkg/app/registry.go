package app

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/kv"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
)

type ObjectRegistry struct {
	debug             bool
	logger            *zap.Logger
	connectionFactory db.ConnectionFactory
	inventory         cluster.Inventory
	kvRepository      *kv.Repository
	initialized       bool
}

func NewObjectRegistry(cf db.ConnectionFactory, debug bool) (*ObjectRegistry, error) {
	or := &ObjectRegistry{
		debug:             debug,
		connectionFactory: cf,
	}
	if err := or.init(); err != nil {
		return nil, err
	}
	return or, nil
}

func (or *ObjectRegistry) init() error {
	if or.initialized {
		return nil
	}
	var err error
	if or.logger, err = logger.NewLogger(or.debug); err != nil {
		return err
	}
	if or.inventory, err = or.initInventory(); err != nil {
		return err
	}
	if or.kvRepository, err = or.initRepository(); err != nil {
		return err
	}
	or.initialized = true
	return nil
}

func (or *ObjectRegistry) Close() error {
	if !or.initialized {
		return nil
	}
	if err := or.kvRepository.Close(); err != nil {
		return err
	}
	return nil
}

func (or *ObjectRegistry) Logger() *zap.Logger {
	return or.logger
}

func (or *ObjectRegistry) Inventory() cluster.Inventory {
	return or.inventory
}

func (or *ObjectRegistry) KVRepository() *kv.Repository {
	return or.kvRepository
}

func (or *ObjectRegistry) initRepository() (*kv.Repository, error) {
	var err error

	var repository *kv.Repository
	if or.connectionFactory == nil {
		or.logger.Fatal("Failed to create configuration entry repository because connection factory is undefined")
	}
	repository, err = kv.NewRepository(or.connectionFactory, or.debug)
	if err != nil {
		or.logger.Error(fmt.Sprintf("Failed to create configuration entry repository: %s", err))
		return nil, err
	}

	return repository, nil
}

func (or *ObjectRegistry) initInventory() (cluster.Inventory, error) {
	var err error

	if or.connectionFactory == nil {
		or.logger.Fatal("Failed to create cluster inventory because connection factory is undefined")
	}
	or.inventory, err = cluster.NewInventory(or.connectionFactory, or.debug)
	if err != nil {
		or.logger.Error(fmt.Sprintf("Failed to create cluster inventory: %s", err))
		return nil, err
	}

	return or.inventory, nil
}
