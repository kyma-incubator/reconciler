package db

import "database/sql"

//Introducing our own interface to be able to add logging capabilities
type Connection interface {
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
	Begin() (*sql.Tx, error)
	Close() error
}

type ConnectionFactory interface {
	NewConnection() (Connection, error)
}
