package repository

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
)

type Repository struct {
	Debug    bool
	Conn     db.Connection
	Logger   *zap.SugaredLogger
	CacheDep *cacheDependencyManager
}

func NewRepository(conn db.Connection, debug bool) (*Repository, error) {
	return &Repository{
		Debug:    debug,
		Conn:     conn,
		Logger:   log.NewLogger(debug),
		CacheDep: newCacheDependencyManager(debug),
	}, nil
}

func (r *Repository) TransactionalResult(dbOps func(tx *db.TxConnection) (interface{}, error)) (interface{}, error) {
	return db.TransactionResult(r.Conn, dbOps, r.Logger)
}

func (r *Repository) Transactional(dbOps func(tx *db.TxConnection) error) error {
	return db.Transaction(r.Conn, dbOps, r.Logger)
}
