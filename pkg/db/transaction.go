package db

import (
	"database/sql"
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync"
)

func TransactionResult(conn Connection, dbOps func(tx *TxConnection) (interface{}, error), logger *zap.SugaredLogger) (interface{}, error) {
	log := func(msg string, args ...interface{}) {
		if logger != nil {
			logger.Debugf(msg, args...)
		}
	}
	txConnection, err := conn.Begin()
	if err != nil {
		return nil, err
	}

	result, err := dbOps(txConnection)
	if err != nil {
		log("Rollback transactional DB context because an error occurred: %s", err)
		if rollbackErr := txConnection.tx.Rollback(); rollbackErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", txConnection.tx.Rollback()))
		}
		return result, err
	}

	return result, txConnection.commit()
}

func Transaction(conn Connection, dbOps func(tx *TxConnection) error, logger *zap.SugaredLogger) error {
	dbOpsAdapter := func(tx *TxConnection) (interface{}, error) {
		return nil, dbOps(tx)
	}
	_, err := TransactionResult(conn, dbOpsAdapter, logger)
	return err
}

type TxConnection struct {
	tx      *sql.Tx
	conn    Connection
	counter uint
	logger  *zap.SugaredLogger
	sync.Mutex
}

func (t *TxConnection) DB() *sql.DB {
	return t.conn.DB()
}

func (t *TxConnection) Encryptor() *Encryptor {
	return t.conn.Encryptor()
}

func (t *TxConnection) Ping() error {
	return t.conn.Ping()
}

func (t *TxConnection) QueryRow(query string, args ...interface{}) (DataRow, error) {
	return t.tx.QueryRow(query, args...), nil
}

func (t *TxConnection) Query(query string, args ...interface{}) (DataRows, error) {
	return t.tx.Query(query, args...)
}

func (t *TxConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}

func (t *TxConnection) Begin() (*TxConnection, error) {
	t.logger.Debug("Transaction Begin")
	t.increaseCounter()
	return t, nil
}

func (t *TxConnection) Close() error {
	return t.conn.Close()
}

func (t *TxConnection) Type() Type {
	return t.conn.Type()
}

func (t *TxConnection) GetTx() *sql.Tx {
	return t.tx
}

func (t *TxConnection) commit() error {
	t.decreaseCounter()
	if t.counter == 0 {
		t.logger.Debug("Transaction Committed")
		return t.tx.Commit()
	}
	return nil
}

func (t *TxConnection) increaseCounter() {
	t.Lock()
	defer t.Unlock()
	t.counter++
}

func (t *TxConnection) decreaseCounter() {
	t.Lock()
	defer t.Unlock()
	t.counter--
}
