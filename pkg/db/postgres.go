package db

import (
	"database/sql"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/logger"

	//add Postgres driver:
	_ "github.com/lib/pq"

	"go.uber.org/zap"
)

type PostgresConnection struct {
	db     *sql.DB
	logger *zap.SugaredLogger
}

func newPostgresConnection(db *sql.DB, debug bool) (*PostgresConnection, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &PostgresConnection{
		db:     db,
		logger: logger,
	}, nil
}

func (pc *PostgresConnection) QueryRow(query string, args ...interface{}) DataRow {
	pc.logger.Debug("Postgres QueryRow(): %s | %v", query, args)
	return pc.db.QueryRow(query, args...)
}

func (pc *PostgresConnection) Query(query string, args ...interface{}) (DataRows, error) {
	pc.logger.Debug("Postgres Query(): %s | %v", query, args)
	rows, err := pc.db.Query(query, args...)
	if err != nil {
		pc.logger.Error("Postgres Query() error: %s", err)
	}
	return rows, err
}

func (pc *PostgresConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	pc.logger.Debug("Postgres Exec(): %s | %v", query, args)
	result, err := pc.db.Exec(query, args...)
	if err != nil {
		pc.logger.Error("Postgres Exec() error: %s", err)
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
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SslMode  bool
	Debug    bool
}

func (pcf *PostgresConnectionFactory) Init() error {
	//no init action required for postgres
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

	return newPostgresConnection(db, pcf.Debug)
}
