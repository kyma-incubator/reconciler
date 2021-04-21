package config

import (
	"database/sql"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const (
	tblKeys = "config_keys"
	// tlbValues          = "config_values"
	tblKeysPKKey     = "key"
	tblKeysPKVersion = "version"
	// tblValuesPKBucket  = "bucket"
	// tblValuesPKKey     = "key"
	// tblValuesPKVersion = "version"
)

type ConfigEntryRepository struct {
	conn db.Connection
}

func NewConfigEntryRepository(dbFac db.ConnectionFactory) (*ConfigEntryRepository, error) {
	conn, err := dbFac.NewConnection()
	return &ConfigEntryRepository{
		conn: conn,
	}, err
}

func (cer *ConfigEntryRepository) GetKeys(key string) ([]*KeyEntity, error) {
	entity := &KeyEntity{}
	colHdlr, err := db.NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}
	rows, err := cer.conn.Query(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1",
			colHdlr.ColumnNamesCsv(false), tblKeys, tblKeysPKKey),
		key)
	if err != nil {
		return nil, err
	}
	return cer.unmarshalKeyEntities(rows)
}

func (cer *ConfigEntryRepository) GetLatestKey(key string) (*KeyEntity, error) {
	entity := &KeyEntity{}
	colHdlr, err := db.NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}
	row := cer.conn.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1 ORDER BY VERSION DESC",
			colHdlr.ColumnNamesCsv(false), tblKeys, tblKeysPKKey),
		key)

	return entity, colHdlr.Synchronize(row, entity)
}

func (cer *ConfigEntryRepository) GetKey(key string, version int64) (*KeyEntity, error) {
	entity := &KeyEntity{}
	colHdlr, err := db.NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}
	row := cer.conn.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1 AND %s=$2",
			colHdlr.ColumnNamesCsv(false), tblKeys, tblKeysPKKey, tblKeysPKVersion),
		key, version)

	return entity, colHdlr.Synchronize(row, entity)
}

func (cer *ConfigEntryRepository) CreateKey(key *KeyEntity) (*KeyEntity, error) {
	colHdlr, err := db.NewColumnHandler(key)
	if err != nil {
		return key, err
	}
	if err := colHdlr.Validate(); err != nil {
		return key, err
	}
	//TODO: check latest key if it's equal with current key
	row := cer.conn.QueryRow(
		fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
			tblKeys, colHdlr.ColumnNamesCsv(true), colHdlr.ColumnValuesPlaceholderCsv(true), colHdlr.ColumnNamesCsv(false)),
		colHdlr.ColumnValues(true)...)

	return key, colHdlr.Synchronize(row, key)
}

func (cer *ConfigEntryRepository) DeleteKey(key *KeyEntity) (int64, error) {
	res, err := cer.conn.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE key = '$1'",
			tblKeys), key.Key)
	if err == nil {
		return res.RowsAffected()
	} else {
		return 0, err
	}
}

func (cer *ConfigEntryRepository) unmarshalKeyEntities(rows *sql.Rows) ([]*KeyEntity, error) {
	result := []*KeyEntity{}
	for rows.Next() {
		entity := &KeyEntity{}
		stc, err := db.NewColumnHandler(entity)
		if err != nil {
			return nil, err
		}
		if err := stc.Synchronize(rows, entity); err != nil {
			return result, err
		}
		result = append(result, entity)
	}
	return result, nil
}

func (cer *ConfigEntryRepository) GetValue(bucket, key string, version int64) (*ValueEntity, error) {
	return nil, nil
}

func (cer *ConfigEntryRepository) CreateValue(key *ValueEntity) error {
	return nil
}

func (cer *ConfigEntryRepository) DeleteValue(key *ValueEntity) error {
	return nil
}

func (cer *ConfigEntryRepository) Close() error {
	return cer.conn.Close()
}
