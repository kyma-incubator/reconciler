package db

import (
	"database/sql"
	"fmt"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/pkg/errors"
	//add Postgres driver:
	_ "github.com/lib/pq"

	"go.uber.org/zap"
)

type PostgresConnection struct {
	db        *sql.DB
	encryptor *Encryptor
	logger    *zap.SugaredLogger
}

func newPostgresConnection(db *sql.DB, encryptionKey string, debug bool) (*PostgresConnection, error) {
	logger, err := log.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	encryptor, err := NewEncryptor(encryptionKey)
	if err != nil {
		return nil, err
	}
	return &PostgresConnection{
		db:        db,
		encryptor: encryptor,
		logger:    logger,
	}, nil
}

func (pc *PostgresConnection) Encryptor() *Encryptor {
	return pc.encryptor
}

func (pc *PostgresConnection) QueryRow(query string, args ...interface{}) DataRow {
	pc.logger.Debugf("Postgres QueryRow(): %s | %v", query, args)
	return pc.db.QueryRow(query, args...)
}

func (pc *PostgresConnection) Query(query string, args ...interface{}) (DataRows, error) {
	pc.logger.Debugf("Postgres Query(): %s | %v", query, args)
	rows, err := pc.db.Query(query, args...)
	if err != nil {
		pc.logger.Errorf("Postgres Query() error: %s", err)
	}
	return rows, err
}

func (pc *PostgresConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	pc.logger.Debugf("Postgres Exec(): %s | %v", query, args)
	result, err := pc.db.Exec(query, args...)
	if err != nil {
		pc.logger.Errorf("Postgres Exec() error: %s", err)
	}
	return result, err
}

func (pc *PostgresConnection) Begin() (*sql.Tx, error) {
	pc.logger.Debug("Postgres Begin()")
	return pc.db.Begin()
}

func (pc *PostgresConnection) Close() error {
	pc.logger.Debug("Postgres Close()")
	return pc.db.Close()
}

func (pc *PostgresConnection) Type() Type {
	return Postgres
}

type PostgresConnectionFactory struct {
	Host          string
	Port          int
	Database      string
	User          string
	Password      string
	SslMode       bool
	EncryptionKey string
	Debug         bool
}

func (pcf *PostgresConnectionFactory) Init() error {
	return pcf.checkPostgresIsolationLevel()
}

func (pcf *PostgresConnectionFactory) NewConnection() (Connection, error) {
	sslMode := "disable"
	if pcf.SslMode {
		sslMode = "require"
	}

	db, err := sql.Open(
		"postgres",
		fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			pcf.Host, pcf.Port, pcf.User, pcf.Password, pcf.Database, sslMode))

	if err == nil {
		err = db.Ping()
	}

	if err != nil {
		return nil, err
	}

	return newPostgresConnection(db, pcf.EncryptionKey, pcf.Debug)
}

func (pcf *PostgresConnectionFactory) checkPostgresIsolationLevel() error {
	logger := log.NewOptionalLogger(pcf.Debug)

	dbConn, err := pcf.NewConnection()
	if err != nil {
		return errors.Wrap(err, "not able to open DB connection to verify DB isolation level")
	}

	defer func() {
		if err := dbConn.Close(); err != nil {
			logger.Warnf("Failed to close DB connection which was used to get Postgres isolation level: %s", err)
		}
	}()

	res, err := dbConn.Query("SHOW TRANSACTION ISOLATION LEVEL")
	if err != nil {
		return errors.Wrap(err, "failed to get isolation level from Postgres DB")
	}

	var isoLevel string
	if res.Next() {
		if err := res.Scan(&isoLevel); err != nil {
			return errors.Wrap(err, "failed to bind Postgres result which includes isolation level")
		}
		if isoLevel == sql.LevelReadUncommitted.String() {
			//stop bootstrapping if isolation level is too low
			return fmt.Errorf("postgres isolation level has to be >= '%s' but was '%s'",
				isoLevel, sql.LevelReadCommitted.String())
		}
	} else {
		return errors.New("Postgres isolation level unknown")
	}

	logger.Infof("Postgres isolation level is: %v", isoLevel)

	return nil
}
