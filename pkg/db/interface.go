package db

import (
	"database/sql"
	"math/rand"
)

type Type string

const (
	Postgres Type = "postgres"
	SQLite   Type = "sqlite"
	Mock     Type = "mock"
)

type Connection interface {
	DB() *sql.DB
	Encryptor() *Encryptor
	Ping() error
	QueryRow(query string, args ...interface{}) (DataRow, error)
	Query(query string, args ...interface{}) (DataRows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
	Begin() (*TxConnection, error)
	Close() error
	Type() Type
	ID() string
}

type ConnectionFactory interface {
	Init(migrate bool) error
	NewConnection() (Connection, error)
}

type DatabaseEntity interface {
	Table() string
	Marshaller() *EntityMarshaller
	New() DatabaseEntity
	Equal(other DatabaseEntity) bool
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

func newID() string {
	const idLength = 5
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	b := make([]rune, idLength)
	for i := range b {
		//nolint:gosec //this code is not used for any security related functionality and linter finding can be ignored
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
