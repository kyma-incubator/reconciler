package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/pkg/errors"

	//add Postgres driver:
	_ "github.com/lib/pq"

	"go.uber.org/zap"
)

type PostgresConnection struct {
	db                *sql.DB
	encryptor         *Encryptor
	logger            *zap.SugaredLogger
	executeUnverified bool
}

func newPostgresConnection(db *sql.DB, encryptionKey string, debug bool, executeUnverified bool) (*PostgresConnection, error) {
	logger, err := log.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	encryptor, err := NewEncryptor(encryptionKey)
	if err != nil {
		return nil, err
	}
	return &PostgresConnection{
		db:                db,
		encryptor:         encryptor,
		logger:            logger,
		executeUnverified: executeUnverified,
	}, nil
}

func (pc *PostgresConnection) Encryptor() *Encryptor {
	return pc.encryptor
}

func (pc *PostgresConnection) QueryRow(query string, args ...interface{}) (DataRow, error) {
	pc.logger.Debugf("Postgres QueryRow(): %s | %v", query, args)
	if !pc.validate(query) {
		pc.logger.Errorf("Regex validation for query '%s' failed", query)
		if !pc.executeUnverified {
			return nil, fmt.Errorf("Regex validation for query '%s' failed", query)
		}
	}
	return pc.db.QueryRow(query, args...), nil
}

func (pc *PostgresConnection) Query(query string, args ...interface{}) (DataRows, error) {
	pc.logger.Debugf("Postgres Query(): %s | %v", query, args)
	if !pc.validate(query) {
		pc.logger.Errorf("Regex validation failed for query '%s'", query)
		if !pc.executeUnverified {
			return nil, fmt.Errorf("Regex validation failed for query '%s'", query)
		}
	}
	rows, err := pc.db.Query(query, args...)
	if err != nil {
		pc.logger.Errorf("Postgres Query() error: %s", err)
	}
	return rows, err
}

func (pc *PostgresConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	pc.logger.Debugf("Postgres Exec(): %s | %v", query, args)
	if !pc.validate(query) {
		pc.logger.Errorf("Regex validation failed for query '%s'", query)
		if !pc.executeUnverified {
			return nil, fmt.Errorf("Regex validation failed for query '%s'", query)
		}
	}
	result, err := pc.db.Exec(query, args...)
	if err != nil {
		pc.logger.Errorf("Postgres Exec() error: %s", err)
	}
	return result, err
}

func (pc *PostgresConnection) validate(query string) bool {
	matchSelect, err := regexp.MatchString("SELECT.*(FROM\\s*\\w+\\s*)+(WHERE (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)*)?(\\w*\\s+IN\\s+[^;]+)?(\\w*\\s+ORDER BY\\s+[^;]+)?(\\w*\\s+GROUP BY\\s+[^;]+)?$", query)
	if err != nil {
		pc.logger.Errorf("Regex pattern error: %s", err)
		return false
	}
	matchInsert, err := regexp.MatchString("INSERT.*VALUES \\((\\$\\d+)(\\s*,\\s*\\$\\d+)*\\)[^;]+$", query)
	if err != nil {
		pc.logger.Errorf("Regex pattern error: %s", err)
		return false
	}
	matchUpdate, err := regexp.MatchString("UPDATE.*SET (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?)+(\\s*WHERE\\s*(\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)+)?$", query)
	if err != nil {
		pc.logger.Errorf("Regex pattern error: %s", err)
		return false
	}
	matchDelete, err := regexp.MatchString("DELETE FROM.*WHERE (\\w*\\s*=\\s*\\$\\d+(\\s*,\\s*)?(\\s+AND\\s+)?(\\s+OR\\s+)?)*(\\w*\\s+IN\\s+[^;]+)?$", query)
	if err != nil {
		pc.logger.Errorf("Regex pattern error: %s", err)
		return false
	}

	matchOthers := strings.Contains(query, "CREATE TABLE") || strings.Contains(query, "SHOW TRANSACTION")

	return matchSelect || matchInsert || matchUpdate || matchDelete || matchOthers
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
	Host              string
	Port              int
	Database          string
	User              string
	Password          string
	SslMode           bool
	EncryptionKey     string
	Debug             bool
	ExecuteUnverified bool
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

	return newPostgresConnection(db, pcf.EncryptionKey, pcf.Debug, pcf.ExecuteUnverified)
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
