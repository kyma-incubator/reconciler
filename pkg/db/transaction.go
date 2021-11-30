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
	err := conn.TxBegin()
	if err != nil {
		return nil, err
	}

	result, err := dbOps()
	if err != nil {
		log("Rollback transactional DB context")
		if rollbackErr := conn.TxRollback(); rollbackErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", rollbackErr))
		}
		return result, err
	}

	log("Commit transactional DB context")
	return result, conn.TxCommit()
}

func Transaction(conn Connection, dbOps func() error, logger *zap.SugaredLogger) error {
	dbOpsAdapter := func() (interface{}, error) {
		return nil, dbOps()
	}
	_, err := TransactionResult(conn, dbOpsAdapter, logger)
	return err
}
