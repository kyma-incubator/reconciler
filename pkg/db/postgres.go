package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type PostgresConnectionFactory struct {
	Host     string
	Database string
	User     string
	Password string
	SslMode  bool
}

func (pcf *PostgresConnectionFactory) NewConnection() (*sql.DB, error) {
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

	return db, nil
}
