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

//scanner introduces a interface for the Scan function to make sql.Row and sql.Rows exchangable
type scanner interface {
	Scan(dest ...interface{}) error
}

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
	tblConv, err := db.NewStructTableConverter(KeyEntity{})
	if err != nil {
		return nil, err
	}
	rows, err := cer.conn.Query(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1",
			tblConv.ColumnNamesCsv(false), tblKeys, tblKeysPKKey),
		key)
	if err != nil {
		return nil, err
	}
	return cer.unmarshalKeyEntities(rows)
}

func (cer *ConfigEntryRepository) GetLatestKey(key string) (*KeyEntity, error) {
	tblConv, err := db.NewStructTableConverter(KeyEntity{})
	if err != nil {
		return nil, err
	}
	row := cer.conn.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1 ORDER BY VERSION DESC",
			tblConv.ColumnNamesCsv(false), tblKeys, tblKeysPKKey),
		key)
	return cer.unmarshalKeyEntity(row)
}

func (cer *ConfigEntryRepository) GetKey(key string, version int64) (*KeyEntity, error) {
	tblConv, err := db.NewStructTableConverter(KeyEntity{})
	if err != nil {
		return nil, err
	}
	row := cer.conn.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1 AND %s=$2",
			tblConv.ColumnNamesCsv(false), tblKeys, tblKeysPKKey, tblKeysPKVersion),
		key, version)
	return cer.unmarshalKeyEntity(row)
}

func (cer *ConfigEntryRepository) CreateKey(key *KeyEntity) error {
	tblConv, err := db.NewStructTableConverter(KeyEntity{})
	if err != nil {
		return err
	}
	//TODO: check latest key if it's equal with current key
	return cer.conn.QueryRow(
		fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING version",
			tblKeys, tblConv.ColumnNamesCsv(true), tblConv.ColumnValuesPlaceholderCsv(true)),
		tblConv.ColumnValues(true)...).Scan(key.Version)
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
		keyEntity, err := cer.unmarshalKeyEntity(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, keyEntity)
	}
	return result, nil
}

func (cer *ConfigEntryRepository) unmarshalKeyEntity(scan scanner) (*KeyEntity, error) {
	return &KeyEntity{}, nil
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
