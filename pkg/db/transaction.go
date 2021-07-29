package db

import (
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func TransactionResult(conn Connection, dbOps func() (interface{}, error), logger *zap.SugaredLogger) (interface{}, error) {
	log := func(msg string) {
		if logger != nil {
			logger.Debug(msg)
		}
	}
	log("Begin transactional DB context")
	tx, err := conn.Begin()
	if err != nil {
		return nil, err
	}

	result, err := dbOps()
	if err != nil {
		log("Rollback transactional DB context")
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", tx.Rollback()))
		}
		return result, err
	}

	log("Commit transactional DB context")
	return result, tx.Commit()
}

func Transaction(conn Connection, dbOps func() error, logger *zap.SugaredLogger) error {
	dbOpsAdapter := func() (interface{}, error) {
		return nil, dbOps()
	}
	_, err := TransactionResult(conn, dbOpsAdapter, logger)
	return err
}
