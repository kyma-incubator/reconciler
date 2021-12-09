package db

import (
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func TransactionResult(conn Connection, dbOps func(tx *Tx) (interface{}, error), logger *zap.SugaredLogger) (interface{}, error) {
	log := func(msg string, args ...interface{}) {
		if logger != nil {
			logger.Debugf(msg, args...)
		}
	}
	transaction, err := conn.Begin()
	if err != nil {
		return nil, err
	}

	result, err := dbOps(transaction)
	if err != nil {
		log("Rollback transactional DB context because an error occurred: %s", err)
		if rollbackErr := transaction.tx.Rollback(); rollbackErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", transaction.tx.Rollback()))
		}
		return result, err
	}

	return result, transaction.tx.Commit()
}

func Transaction(conn Connection, dbOps func(tx *Tx) error, logger *zap.SugaredLogger) error {
	dbOpsAdapter := func(tx *Tx) (interface{}, error) {
		return nil, dbOps(tx)
	}
	_, err := TransactionResult(conn, dbOpsAdapter, logger)
	return err
}

type Tx struct {
	tx   *sql.Tx
	conn Connection
}

func (t *Tx) DB() *sql.DB {
	return t.conn.DB()
}

func (t *Tx) Encryptor() *Encryptor {
	return t.conn.Encryptor()
}

func (t *Tx) Ping() error {
	return t.conn.Ping()
}

func (t *Tx) QueryRow(query string, args ...interface{}) (DataRow, error) {
	return t.tx.QueryRow(query, args...), nil
}

func (t *Tx) Query(query string, args ...interface{}) (DataRows, error) {
	return t.tx.Query(query, args...)
}

func (t *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}

func (t *Tx) Begin() (*sql.Tx, error) {
	return t.tx, nil
}

func (t *Tx) Close() error {
	return t.conn.Close()
}

func (t *Tx) Type() Type {
	return t.conn.Type()
}
