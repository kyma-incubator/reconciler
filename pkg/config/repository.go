package config

import (
	"database/sql"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type ConfigEntryRepository struct {
	db *sql.DB
}

func NewConfigEntryRepository(dbFac db.ConnectionFactory) (*ConfigEntryRepository, error) {
	db, err := dbFac.NewConnection()
	return &ConfigEntryRepository{
		db: db,
	}, err
}

func (cer *ConfigEntryRepository) Close() error {
	return cer.db.Close()
}
