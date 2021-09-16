package db

import (
	"database/sql"
	"fmt"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/pkg/errors"

	//add Postgres driver:
	_ "github.com/lib/pq"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

type PostgresConnection struct {
	db        *sql.DB
	encryptor *Encryptor
	validator *Validator
	logger    *zap.SugaredLogger
}

func newPostgresConnection(db *sql.DB, encryptionKey string, debug bool, blockQueries bool) (*PostgresConnection, error) {
	logger := log.NewLogger(debug)

	encryptor, err := NewEncryptor(encryptionKey)
	if err != nil {
		return nil, err
	}

	validator := NewValidator(blockQueries, logger)

	return &PostgresConnection{
		db:        db,
		encryptor: encryptor,
		validator: validator,
		logger:    logger,
	}, nil
}

func (pc *PostgresConnection) DB() *sql.DB {
	return pc.db
}

func (pc *PostgresConnection) Encryptor() *Encryptor {
	return pc.encryptor
}

func (pc *PostgresConnection) QueryRow(query string, args ...interface{}) (DataRow, error) {
	pc.logger.Debugf("Postgres QueryRow(): %s | %v", query, args)
	if err := pc.validator.Validate(query); err != nil {
		return nil, err
	}
	return pc.db.QueryRow(query, args...), nil
}

func (pc *PostgresConnection) Query(query string, args ...interface{}) (DataRows, error) {
	pc.logger.Debugf("Postgres Query(): %s | %v", query, args)
	if err := pc.validator.Validate(query); err != nil {
		return nil, err
	}
	rows, err := pc.db.Query(query, args...)
	if err != nil {
		pc.logger.Errorf("Postgres Query() error: %s", err)
	}
	return rows, err
}

func (pc *PostgresConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	pc.logger.Debugf("Postgres Exec(): %s | %v", query, args)
	if err := pc.validator.Validate(query); err != nil {
		return nil, err
	}
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
	MigrationsDir string
	Debug         bool
	blockQueries  bool
}

func (pcf *PostgresConnectionFactory) Init(migrate bool) error {
	if err := pcf.checkPostgresIsolationLevel(); err != nil {
		return err
	}
	if migrate {
		if err := pcf.migrateDatabase(); err != nil {
			return err
		}
	}
	return nil
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

	return newPostgresConnection(db, pcf.EncryptionKey, pcf.Debug, pcf.blockQueries)
}

func (pcf *PostgresConnectionFactory) checkPostgresIsolationLevel() error {
	logger := log.NewLogger(pcf.Debug)

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

func (pcf *PostgresConnectionFactory) migrateDatabase() error {
	logger := log.NewLogger(pcf.Debug)
	dbConn, err := pcf.NewConnection()
	if err != nil {
		return errors.Wrap(err, "not able to open DB connection to perform migration")
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			logger.Warnf("Failed to close DB connection which was used to perform migration: %s", err)
		}
	}()
	driver, err := postgres.WithInstance(dbConn.DB(), &postgres.Config{})
	if err != nil {
		return errors.Wrap(err, "not able to instantiate postgres driver for migration")
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+pcf.MigrationsDir, "postgres", driver)
	if err != nil {
		return errors.Wrap(err, "not able to instantiate migrator with database instance")
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return errors.Wrapf(err, "not able to execute migrations: %s", err)
	}
	logger.Info("Database migrated")
	return nil
}
