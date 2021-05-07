package db

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

type SqliteConnection struct {
	db     *sql.DB
	logger *zap.Logger
}

func newSqliteConnection(db *sql.DB, debug bool) (*SqliteConnection, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &SqliteConnection{
		db:     db,
		logger: logger,
	}, nil
}

func (sc *SqliteConnection) QueryRow(query string, args ...interface{}) DataRow {
	sc.logger.Debug(fmt.Sprintf("Sqlite3 QueryRow(): %s | %v", query, args))
	return sc.db.QueryRow(query, args...)
}
func (sc *SqliteConnection) Query(query string, args ...interface{}) (DataRows, error) {
	sc.logger.Debug(fmt.Sprintf("Sqlite3 Query(): %s | %v", query, args))
	rows, err := sc.db.Query(query, args...)
	if err != nil {
		sc.logger.Error(fmt.Sprintf("Sqlite3 Query() error: %s", err))
	}
	return rows, err
}
func (sc *SqliteConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	sc.logger.Debug(fmt.Sprintf("Sqlite3 Exec(): %s | %v", query, args))
	result, err := sc.db.Exec(query, args...)
	if err != nil {
		sc.logger.Error(fmt.Sprintf("Sqlite3 Exec() error: %s", err))
	}
	return result, err
}
func (sc *SqliteConnection) Begin() (*sql.Tx, error) {
	sc.logger.Debug("Sqlite3 Begin()")
	return sc.db.Begin()
}
func (sc *SqliteConnection) Close() error {
	sc.logger.Debug("Sqlite3 Close()")
	return sc.db.Close()
}

type SqliteConnectionFactory struct {
	File         string
	Debug        bool
	fileResetted bool
	mutex        sync.Mutex
}

func (scf *SqliteConnectionFactory) NewConnection() (Connection, error) {
	//ensure file is new when first connection is created
	scf.mutex.Lock()
	if !scf.fileResetted {
		if err := scf.resetFile(); err != nil {
			return nil, err
		}
		scf.fileResetted = true
	}
	scf.mutex.Unlock()

	db, err := sql.Open("sqlite3", scf.File) //establish connection
	if err != nil {
		return nil, err
	}

	err = db.Ping() //test connection
	if err != nil {
		return nil, err
	}

	return newSqliteConnection(db, scf.Debug) //connection ready to use
}

func (scf *SqliteConnectionFactory) resetFile() error {
	os.Remove(scf.File)
	file, err := os.Create(scf.File)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}
