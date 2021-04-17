package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type PostgresConnection struct {
	db *sql.DB
}

func (pc *PostgresConnection) Query(sql string) interface{} {
	return nil
}
func (pc *PostgresConnection) Insert(sql string) (int, error) {
	return 0, nil
}
func (pc *PostgresConnection) Update(sql string) (int, error) {
	return 0, nil
}
func (pc *PostgresConnection) Delete(sql string) (int, error) {
	return 0, nil
}
func (pc *PostgresConnection) Close() error {
	return pc.db.Close()
}

type PostgresConnectionFactory struct {
	Host     string
	Database string
	User     string
	Password string
	SslMode  bool
}

func (pcf *PostgresConnectionFactory) NewConnection() (Connection, error) {
	sslMode := "disabled"
	if pcf.SslMode {
		sslMode = "enabled"
	}

	db, err := sql.Open(
		"postgres",
		fmt.Sprintf("user=%s password=%s dbname=%s sslmode=%s", pcf.User, pcf.Password, pcf.Database, sslMode))
	if err != nil {
		return nil, err
	}

	return &PostgresConnection{
		db: db,
	}, nil
}
