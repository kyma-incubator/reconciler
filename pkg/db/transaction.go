package db

import (
	"database/sql"
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const txMaxRetries = 5
const txMaxJitter = 350
const txMinJitter = 25

func TransactionResult(conn Connection, dbOps func(tx *TxConnection) (interface{}, error), logger *zap.SugaredLogger) (interface{}, error) {
	var result interface{}
	var err error
	var allErr error

	for retries := 0; retries < txMaxRetries; retries++ {
		result, err = execTransaction(conn, dbOps, logger)
		if err == nil {
			if retries > 0 {
				logger.Debugf("DB transaction after %d retries successfully finished", retries)
			}
			break //all good: leave retry loop
		}

		//chain all retrieved TX errors for better debugging
		if allErr == nil {
			allErr = err
		} else {
			allErr = errors.Wrap(allErr, err.Error())
		}

		if isAlreadyCommitedOrRolledBackError(err) { //TX is already closed: give up
			break
		}

		if isCollidingTxError(err) { //TX collided: retry
			delay := randomJitter()
			logger.Debugf("DB transaction collision occurred and transaction will be retried in %d msec",
				delay.Milliseconds())
			time.Sleep(delay)
			continue
		}

		if retries > 0 {
			logger.Debugf("DB transaction failed non-recoverable in retry #%d: %s", retries, err)
		}

		break //anything else went wrong: give up
	}

	return result, allErr
}

func randomJitter() time.Duration {
	rand.Seed(time.Now().UnixNano())
	//nolint:gosec //no security relevance, linter complains can be ignored
	jitter := rand.Int63n(txMaxJitter)
	if jitter < txMinJitter {
		jitter = jitter + txMinJitter
	}
	return time.Duration(jitter) * time.Millisecond
}

func execTransaction(conn Connection, dbOps func(tx *TxConnection) (interface{}, error), logger *zap.SugaredLogger) (interface{}, error) {
	log := func(msg string, args ...interface{}) {
		if logger != nil {
			logger.Infof(msg, args...)
		}
	}

	txConnection, err := conn.Begin()
	if err != nil {
		return nil, err
	}

	result, err := dbOps(txConnection)
	if err != nil {
		log("Rollback transactional DB context because an error occurred: %s", err)
		if rollbackErr := txConnection.Rollback(); rollbackErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("Rollback of db operation failed: %s", rollbackErr))
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

func isCollidingTxError(err error) bool {
	return strings.Contains(err.Error(), "could not serialize access")
}

func isAlreadyCommitedOrRolledBackError(err error) bool {
	return strings.Contains(err.Error(), "already been committed or rolled back")
}
