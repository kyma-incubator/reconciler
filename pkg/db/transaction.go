package db

import (
	"database/sql"
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

const txMaxRetries = 3
const txMaxJitter = 350
const txMinJitter = 25

func TransactionResult(conn Connection, dbOps func(tx *TxConnection) (interface{}, error), logger *zap.SugaredLogger) (interface{}, error) {
	var result interface{}
	var txErr error

	log := func(msg string, args ...interface{}) {
		if logger == nil {
			return
		}
		logger.Infof(msg, args...)
	}

	//retry the transaction until txMaxRetries is reached. Each retry has a few msec delay for better load balancing
	for i := 0; i < txMaxRetries; i++ {
		if i > 0 {
			rand.Seed(time.Now().UnixNano())
			//nolint:gosec //no security relevance, linter complains can be ignored
			jitter := time.Duration(rand.Int63n(txMaxJitter))
			if jitter < txMinJitter {
				jitter = jitter + txMinJitter
			}
			time.Sleep(jitter)
		}

		txConnection, err := conn.Begin()
		if err != nil {
			return nil, err
		}

		result, err = dbOps(txConnection)
		if err != nil {
			log("Rollback transactional DB context because an error occurred: %s", err)
			if rollbackErr := txConnection.Rollback(); rollbackErr != nil {
				err = errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", rollbackErr))
			}
			return result, err
		}

		commitErr := txConnection.commit()
		if commitErr == nil {
			return result, nil
		}

		txErr = errors.Wrap(txErr, commitErr.Error())
		log("Rollback transactional DB context because commit failed: %s", commitErr.Error())
		if txRollbackErr := txConnection.Rollback(); txRollbackErr != nil {
			txErr = errors.Wrap(txErr, txRollbackErr.Error())
		}

	}

	return result, txErr
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

func NewTxConnection(tx *sql.Tx, conn Connection, logger *zap.SugaredLogger) *TxConnection {
	//setting counter to 1 since first begin is not called with counter increase
	return &TxConnection{tx: tx, conn: conn, counter: 1, logger: logger}
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

func (t *TxConnection) Rollback() error {
	return t.tx.Rollback()
}
