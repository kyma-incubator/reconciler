package db

import (
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func TransactionResult(conn Connection, dbOps func() (interface{}, error), logger *zap.SugaredLogger) (interface{}, error) {
	log := func(msg string, args ...interface{}) {
		if logger != nil {
			logger.Debugf(msg, args...)
		}
	}
	tx, err := conn.Begin()
	if err != nil {
		return nil, err
	}

	result, err := dbOps()
	if err != nil {
		log("Rollback transactional DB context because an error occurred: %s", err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", tx.Rollback()))
		}
		return result, err
	}

	return result, tx.Commit()
}

func Transaction(conn Connection, dbOps func() error, logger *zap.SugaredLogger) error {
	dbOpsAdapter := func() (interface{}, error) {
		return nil, dbOps()
	}
	_, err := TransactionResult(conn, dbOpsAdapter, logger)
	return err
}
