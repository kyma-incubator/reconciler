package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	log "github.com/kyma-incubator/reconciler/pkg/logger"

	//add SQlite driver:
	_ "github.com/mattn/go-sqlite3"

	"go.uber.org/zap"
)

type SqliteConnection struct {
	db                *sql.DB
	encryptor         *Encryptor
	logger            *zap.SugaredLogger
	executeUnverified bool
}

func newSqliteConnection(db *sql.DB, encKey string, debug bool, executeUnverified bool) (*SqliteConnection, error) {
	logger, err := log.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	encryptor, err := NewEncryptor(encKey)
	if err != nil {
		return nil, err
	}
	return &SqliteConnection{
		db:                db,
		encryptor:         encryptor,
		logger:            logger,
		executeUnverified: executeUnverified,
	}, nil
}

func (sc *SqliteConnection) Encryptor() *Encryptor {
	return sc.encryptor
}

func (sc *SqliteConnection) QueryRow(query string, args ...interface{}) (DataRow, error) {
	sc.logger.Debugf("Sqlite3 QueryRow(): %s | %v", query, args)
	if !sc.validate(query) {
		sc.logger.Errorf("Regex validation for query '%s' failed", query)
		if !sc.executeUnverified {
			return nil, fmt.Errorf("Regex validation for query '%s' failed", query)
		}
	}
	return sc.db.QueryRow(query, args...), nil
}

func (sc *SqliteConnection) Query(query string, args ...interface{}) (DataRows, error) {
	sc.logger.Debugf("Sqlite3 Query(): %s | %v", query, args)
	if !sc.validate(query) {
		sc.logger.Errorf("Regex validation for query '%s' failed", query)
		if !sc.executeUnverified {
			return nil, fmt.Errorf("Regex validation for query '%s' failed", query)
		}
	}
	rows, err := sc.db.Query(query, args...)
	if err != nil {
		sc.logger.Errorf("Sqlite3 Query() error: %s", err)
	}
	return rows, err
}

func (sc *SqliteConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	sc.logger.Debugf("Sqlite3 Exec(): %s | %v", query, args)
	if !sc.validate(query) {
		sc.logger.Errorf("Regex validation for query '%s' failed", query)
		if !sc.executeUnverified {
			return nil, fmt.Errorf("Regex validation for query '%s' failed", query)
		}
	}
	result, err := sc.db.Exec(query, args...)
	if err != nil {
		sc.logger.Errorf("Sqlite3 Exec() error: %s", err)
	}
	return result, err
}

func (sc *SqliteConnection) validate(query string) bool {
	matchSelect, err := regexp.MatchString("SELECT.*(FROM\\s*\\w+\\s*)+(WHERE (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)*)?(\\w*\\s+IN\\s+[^;]+)?(\\w*\\s+ORDER BY\\s+[^;]+)?(\\w*\\s+GROUP BY\\s+[^;]+)?$", query)
	if err != nil {
		sc.logger.Errorf("Regex validation failed: %s", err)
		return false
	}
	matchInsert, err := regexp.MatchString("INSERT.*VALUES \\((\\$\\d+)(\\s*,\\s*\\$\\d+)*\\)[^;]+$", query)
	if err != nil {
		sc.logger.Errorf("Regex validation failed: %s", err)
		return false
	}
	matchUpdate, err := regexp.MatchString("UPDATE.*SET (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?)+(\\s*WHERE\\s*(\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)+)?$", query)
	if err != nil {
		sc.logger.Errorf("Regex validation failed: %s", err)
		return false
	}
	matchDelete, err := regexp.MatchString("DELETE FROM.*WHERE (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)*(\\w*\\s+IN\\s+[^;]+)?$", query)
	if err != nil {
		sc.logger.Errorf("Regex validation failed: %s", err)
		return false
	}

	matchCreate := strings.Contains(query, "CREATE TABLE")

	return matchSelect || matchInsert || matchUpdate || matchDelete || matchCreate
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
	File              string
	Debug             bool
	Reset             bool
	SchemaFile        string
	EncryptionKey     string
	ExecuteUnverified bool
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

	return newSqliteConnection(db, scf.EncryptionKey, scf.Debug, scf.ExecuteUnverified) //connection ready to use
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
