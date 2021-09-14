package db

import (
	"database/sql"
	"io/ioutil"
	"os"

	log "github.com/kyma-incubator/reconciler/pkg/logger"

	//add SQlite driver:
	_ "github.com/mattn/go-sqlite3"

	"go.uber.org/zap"
)

type SqliteConnection struct {
	db        *sql.DB
	encryptor *Encryptor
	validator *Validator
	logger    *zap.SugaredLogger
}

func newSqliteConnection(db *sql.DB, encKey string, debug bool, blockQueries bool) (*SqliteConnection, error) {
	logger := log.NewLogger(debug)

	encryptor, err := NewEncryptor(encKey)
	if err != nil {
		return nil, err
	}

	validator := NewValidator(blockQueries, logger)

	return &SqliteConnection{
		db:        db,
		encryptor: encryptor,
		validator: validator,
		logger:    logger,
	}, nil
}

func (sc *SqliteConnection) Encryptor() *Encryptor {
	return sc.encryptor
}

func (sc *SqliteConnection) QueryRow(query string, args ...interface{}) (DataRow, error) {
	sc.logger.Debugf("Sqlite3 QueryRow(): %s | %v", query, args)
	if err := sc.validator.Validate(query); err != nil {
		return nil, err
	}
	return sc.db.QueryRow(query, args...), nil
}

func (sc *SqliteConnection) Query(query string, args ...interface{}) (DataRows, error) {
	sc.logger.Debugf("Sqlite3 Query(): %s | %v", query, args)
	if err := sc.validator.Validate(query); err != nil {
		return nil, err
	}
	rows, err := sc.db.Query(query, args...)
	if err != nil {
		sc.logger.Errorf("Sqlite3 Query() error: %s", err)
	}
	return rows, err
}

func (sc *SqliteConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	sc.logger.Debugf("Sqlite3 Exec(): %s | %v", query, args)
	if err := sc.validator.Validate(query); err != nil {
		return nil, err
	}
	result, err := sc.db.Exec(query, args...)
	if err != nil {
		sc.logger.Errorf("Sqlite3 Exec() error: %s", err)
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

func (sc *SqliteConnection) Type() Type {
	return SQLite
}

type SqliteConnectionFactory struct {
	File          string
	Debug         bool
	Reset         bool
	SchemaFile    string
	EncryptionKey string
	blockQueries  bool
}

func (scf *SqliteConnectionFactory) Init() error {
	if scf.Reset {
		if err := scf.resetFile(); err != nil {
			return err
		}
	}
	if scf.SchemaFile != "" {
		//read DDL (test-table structure)
		ddl, err := ioutil.ReadFile(scf.SchemaFile)
		if err != nil {
			return err
		}

		//get connection
		conn, err := scf.NewConnection()
		if err != nil {
			return err
		}

		//populate DB schema
		_, err = conn.Exec(string(ddl))
		return err
	}
	return nil
}

func (scf *SqliteConnectionFactory) NewConnection() (Connection, error) {
	db, err := sql.Open("sqlite3", scf.File) //establish connection
	if err != nil {
		return nil, err
	}

	err = db.Ping() //test connection
	if err != nil {
		return nil, err
	}

	return newSqliteConnection(db, scf.EncryptionKey, scf.Debug, scf.blockQueries) //connection ready to use
}

func (scf *SqliteConnectionFactory) resetFile() error {
	if err := os.Remove(scf.File); err != nil && !os.IsNotExist(err) {
		//errors are ok if file was missing, but other errors are not expected
		return err
	}
	file, err := os.Create(scf.File)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}
