package db

import "database/sql"

//Introducing our own interface to be able to add logging capabilities
//and make testing simpler (allows injection of mocks)
type Connection interface {
	QueryRow(query string, args ...interface{}) DataRow
	Query(query string, args ...interface{}) (DataRows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
	Begin() (*sql.Tx, error)
	Close() error
}

type ConnectionFactory interface {
	NewConnection() (Connection, error)
}

type DatabaseEntity interface {
	Table() string
	Synchronizer() *EntitySynchronizer
	New() DatabaseEntity
}

//DataRow introduces a interface which is implemented by sql.Row and sql.Rows
//to make both usable for retrieving raw data
type DataRow interface {
	Scan(dest ...interface{}) error
}

type DataRows interface {
	Scan(dest ...interface{}) error
	Next() bool
}
