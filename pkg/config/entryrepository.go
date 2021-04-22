package config

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
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
	q, err := db.NewQuery(cer.conn, &KeyEntity{})
	if err != nil {
		return nil, err
	}
	entities, err := q.Select().Where("key=$1", key).GetMany()
	if err != nil {
		return nil, err
	}
	//cast to specific entity
	result := []*KeyEntity{}
	for _, entity := range entities {
		result = append(result, entity.(*KeyEntity))
	}
	return result, nil
}

func (cer *ConfigEntryRepository) GetLatestKey(key string) (*KeyEntity, error) {
	entity := &KeyEntity{}
	colHdlr, err := db.NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}
	row := cer.conn.QueryRow(
		fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1 ORDER BY VERSION DESC",
			colHdlr.ColumnNamesCsv(false), tblKeys, "key"),
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
			colHdlr.ColumnNamesCsv(false), tblKeys, "key", "version"),
		key, version)

	return entity, colHdlr.Synchronize(row, entity)
}

func (cer *ConfigEntryRepository) CreateKey(key *KeyEntity) (*KeyEntity, error) {
	q, err := db.NewQuery(cer.conn, key)
	if err != nil {
		return nil, err
	}
	//TODO: check latest key if it's equal with current key
	return key, q.Insert().Exec()
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
