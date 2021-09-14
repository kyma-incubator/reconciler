package repository

import (
	"bytes"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
)

type Repository struct {
	Conn     db.Connection
	Logger   *zap.SugaredLogger
	CacheDep *cacheDependencyManager
}

func NewRepository(dbFac db.ConnectionFactory, debug bool) (*Repository, error) {
	conn, err := dbFac.NewConnection()
	if err != nil {
		return nil, err
	}

	cacheDepMgr, err := newCacheDependencyManager(conn, debug)
	if err != nil {
		return nil, err
	}

	return &Repository{
		Conn:     conn,
		Logger:   log.NewLogger(debug),
		CacheDep: cacheDepMgr,
	}, nil
}

func (r *Repository) TransactionalResult(dbOps func() (interface{}, error)) (interface{}, error) {
	return db.TransactionResult(r.Conn, dbOps, r.Logger)
}

func (r *Repository) Transactional(dbOps func() error) error {
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *Repository) NewNotFoundError(err error, entity db.DatabaseEntity,
	identifier map[string]interface{}) error {
	return &EntityNotFoundError{
		entity:     entity,
		identifier: identifier,
		err:        err,
	}
}

type EntityNotFoundError struct {
	entity     db.DatabaseEntity
	identifier map[string]interface{}
	err        error
}

func (e *EntityNotFoundError) Error() string {
	var idents bytes.Buffer
	if e.identifier != nil {
		for k, v := range e.identifier {
			if idents.Len() > 0 {
				idents.WriteRune(',')
			}
			idents.WriteString(fmt.Sprintf("%s=%v", k, v))
		}
	}
	return fmt.Sprintf("Entity of type '%T' with identifier '%v' not found", e.entity, idents.String())
}

func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*EntityNotFoundError)
	return ok
}
