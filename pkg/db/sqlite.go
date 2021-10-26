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

type sqliteConnection struct {
	db        *sql.DB
	encryptor *Encryptor
	validator *Validator
	logger    *zap.SugaredLogger
}

func newSqliteConnection(db *sql.DB, encKey string, debug bool, blockQueries bool) (*sqliteConnection, error) {
	logger := log.NewLogger(debug)

	encryptor, err := NewEncryptor(encKey)
	if err != nil {
		return nil, err
	}

	validator := NewValidator(blockQueries, logger)

	return &sqliteConnection{
		db:        db,
		encryptor: encryptor,
		validator: validator,
		logger:    logger,
	}, nil
}

func (sc *sqliteConnection) DB() *sql.DB {
	return sc.db
}

func (sc *sqliteConnection) Encryptor() *Encryptor {
	return sc.encryptor
}

func (sc *sqliteConnection) QueryRow(query string, args ...interface{}) (DataRow, error) {
	sc.logger.Debugf("Sqlite3 QueryRow(): %s | %v", query, args)
	if err := sc.validator.Validate(query); err != nil {
		return nil, err
	}
	return sc.db.QueryRow(query, args...), nil
}

func (sc *sqliteConnection) Query(query string, args ...interface{}) (DataRows, error) {
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

func (sc *sqliteConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
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

func (sc *sqliteConnection) Begin() (*sql.Tx, error) {
	sc.logger.Debug("Sqlite3 Begin()")
	return sc.db.Begin()
}

func (sc *sqliteConnection) Close() error {
	sc.logger.Debug("Sqlite3 Close()")
	return sc.db.Close()
}

func (sc *sqliteConnection) Type() Type {
	return SQLite
}

type sqliteConnectionFactory struct {
	file          string
	debug         bool
	reset         bool
	schemaFile    string
	encryptionKey string
	blockQueries  bool
	logQueries    bool
}

func (scf *sqliteConnectionFactory) Init(_ bool) error {
	if scf.reset {
		if err := scf.resetFile(); err != nil {
			return err
		}
	}
	if scf.schemaFile != "" {
		//read DDL (test-table structure)
		ddl, err := ioutil.ReadFile(scf.schemaFile)
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

func (scf *sqliteConnectionFactory) NewConnection() (Connection, error) {
	db, err := sql.Open("sqlite3", scf.file) //establish connection
	if err != nil {
		return nil, err
	}

	err = db.Ping() //test connection
	if err != nil {
		return nil, err
	}

	return newSqliteConnection(db, scf.encryptionKey, scf.logQueries, scf.blockQueries) //connection ready to use
}

func (scf *sqliteConnectionFactory) resetFile() error {
	if err := os.Remove(scf.file); err != nil && !os.IsNotExist(err) {
		//errors are ok if file was missing, but other errors are not expected
		return err
	}
	file, err := os.Create(scf.file)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}
