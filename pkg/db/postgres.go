package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type PostgresConnection struct {
	db     *sql.DB
	logger *zap.Logger
}

func newPostgresConnection(db *sql.DB, debug bool) (*PostgresConnection, error) {
	var logger *zap.Logger
	var err error
	if debug {
		logger, err = zap.NewDevelopment()
		if err != nil {
			return nil, err
		}
	} else {
		logger = zap.NewNop()
	}
	return &PostgresConnection{
		db:     db,
		logger: logger,
	}, nil
}

func (pc *PostgresConnection) QueryRow(query string, args ...interface{}) *sql.Row {
	pc.logger.Debug(fmt.Sprintf("Postgres QueryRow(): %s | %v", query, args))
	return pc.db.QueryRow(query, args...)
}
func (pc *PostgresConnection) Query(query string, args ...interface{}) (*sql.Rows, error) {
	pc.logger.Debug(fmt.Sprintf("Postgres Query(): %s | %v", query, args))
	return pc.db.Query(query, args...)
}
func (pc *PostgresConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	pc.logger.Debug(fmt.Sprintf("Postgres Exec(): %s | %v", query, args))
	return pc.db.Exec(query, args...)
}
func (pc *PostgresConnection) Begin() (*sql.Tx, error) {
	pc.logger.Debug("Postgres Begin()")
	return pc.db.Begin()
}
func (pc *PostgresConnection) Close() error {
	pc.logger.Debug("Postgres Close()")
	return pc.db.Close()
}

type PostgresConnectionFactory struct {
	Host     string
	Database string
	User     string
	Password string
	SslMode  bool
	Debug    bool
}

func (pcf *PostgresConnectionFactory) NewConnection() (Connection, error) {
	sslMode := "disable"
	if pcf.SslMode {
		sslMode = "require"
	}

	db, err := sql.Open(
		"postgres",
		fmt.Sprintf("user=%s password=%s dbname=%s sslmode=%s", pcf.User, pcf.Password, pcf.Database, sslMode))

	if err == nil {
		err = db.Ping()
	}

	if err != nil {
		return nil, err
	}

	return newPostgresConnection(db, pcf.Debug)
}
