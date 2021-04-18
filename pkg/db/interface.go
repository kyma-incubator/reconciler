package db

import "database/sql"

type ConnectionFactory interface {
	NewConnection() (*sql.DB, error)
}
