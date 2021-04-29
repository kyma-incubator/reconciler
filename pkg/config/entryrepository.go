package config

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/pkg/errors"
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

func (cer *ConfigEntryRepository) Keys(key string) ([]*KeyEntity, error) {
	entity := &KeyEntity{}
	q, err := db.NewQuery(cer.conn, entity)
	if err != nil {
		return nil, err
	}
	entities, err := q.Select().
		Where(map[string]interface{}{"Key": key}).
		OrderBy(map[string]string{"Version": "ASC"}).
		GetMany()
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

func (cer *ConfigEntryRepository) LatestKey(key string) (*KeyEntity, error) {
	q, err := db.NewQuery(cer.conn, &KeyEntity{})
	if err != nil {
		return nil, err
	}
	entity, err := q.Select().
		Where(map[string]interface{}{"Key": key}).
		OrderBy(map[string]string{"Version": "DESC"}).
		Limit(1).
		GetOne()
	if err != nil {
		return nil, err
	}
	return entity.(*KeyEntity), nil
}

func (cer *ConfigEntryRepository) Key(key string, version int64) (*KeyEntity, error) {
	q, err := db.NewQuery(cer.conn, &KeyEntity{})
	if err != nil {
		return nil, err
	}
	entity, err := q.Select().
		Where(map[string]interface{}{"Key": key, "Version": version}).
		GetOne()
	if err != nil {
		return nil, err
	}
	return entity.(*KeyEntity), nil
}

func (cer *ConfigEntryRepository) CreateKey(key *KeyEntity) (*KeyEntity, error) {
	q, err := db.NewQuery(cer.conn, key)
	if err != nil {
		return nil, err
	}
	//TODO create only if not equal!
	return key, q.Insert().Exec()
}

func (cer *ConfigEntryRepository) DeleteKey(key *KeyEntity) error {
	//bundle DB operations
	dbOps := func() error {
		if err := cer.deleteValuesByKey(key); err != nil {
			return err
		}
		//delete all values mapped to the key
		qKey, err := db.NewQuery(cer.conn, key)
		if err != nil {
			return err
		}
		deleted, err := qKey.Delete().
			Where(map[string]interface{}{"Key": key.Key, "Version": key.Version}).
			Exec()
		if deleted > 1 {
			return fmt.Errorf(
				"Data inconsistency detected when deleting key '%s'. Expected max 1 deletion but deleted '%d' entities",
				key, deleted)
		}
		return err
	}

	//run db-operations transactional
	tx, err := cer.conn.Begin()
	if err != nil {
		return err
	}
	if err := dbOps(); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", tx.Rollback()))
	}
	return tx.Commit()
}

func (cer *ConfigEntryRepository) Values(bucket, key string) ([]*ValueEntity, error) {
	entity := &ValueEntity{}
	q, err := db.NewQuery(cer.conn, entity)
	if err != nil {
		return nil, err
	}
	entities, err := q.Select().
		Where(map[string]interface{}{"Bucket": bucket, "Key": key}).
		OrderBy(map[string]string{"Version": "ASC"}).
		GetMany()
	if err != nil {
		return nil, err
	}
	//cast to specific entity
	result := []*ValueEntity{}
	for _, entity := range entities {
		result = append(result, entity.(*ValueEntity))
	}
	return result, nil
}

func (cer *ConfigEntryRepository) LatestValue(bucket, key string) (*ValueEntity, error) {
	q, err := db.NewQuery(cer.conn, &ValueEntity{})
	if err != nil {
		return nil, err
	}
	entity, err := q.Select().
		Where(map[string]interface{}{"Key": key, "Bucket": bucket}).
		OrderBy(map[string]string{"Version": "DESC"}).
		Limit(1).
		GetOne()
	if err != nil {
		return nil, err
	}
	return entity.(*ValueEntity), nil
}

func (cer *ConfigEntryRepository) Value(bucket, key string, version int64) (*ValueEntity, error) {
	q, err := db.NewQuery(cer.conn, &ValueEntity{})
	if err != nil {
		return nil, err
	}
	entity, err := q.Select().
		Where(map[string]interface{}{"Bucket": bucket, "Key": key, "Version": version}).
		GetOne()
	if err != nil {
		return nil, err
	}
	return entity.(*ValueEntity), nil
}

func (cer *ConfigEntryRepository) CreateValue(value *ValueEntity) (*ValueEntity, error) {
	q, err := db.NewQuery(cer.conn, value)
	if err != nil {
		return nil, err
	}
	return value, q.Insert().Exec()
}

func (cer *ConfigEntryRepository) deleteValuesByKey(key *KeyEntity) error {
	q, err := db.NewQuery(cer.conn, &ValueEntity{})
	if err != nil {
		return err
	}
	_, err = q.Delete().
		Where(map[string]interface{}{"Key": key.Key, "KeyVersion": key.Version}).
		Exec()
	return err
}

func (cer *ConfigEntryRepository) Buckets() ([]*BucketEntity, error) {
	bucketNames, err := cer.bucketNames()
	if err != nil {
		return nil, err
	}

	buckets := []*BucketEntity{}
	for _, bucketName := range bucketNames {
		q, err := db.NewQuery(cer.conn, &BucketEntity{})
		if err != nil {
			return nil, err
		}
		entity, err := q.Select().
			Where(map[string]interface{}{"Bucket": bucketName}).
			OrderBy(map[string]string{"Created": "ASC"}).
			Limit(1).
			GetOne()
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, entity.(*BucketEntity))
	}

	return buckets, nil
}

func (cer *ConfigEntryRepository) bucketNames() ([]string, error) {
	entity := &BucketEntity{}

	colHdlr, err := db.NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}

	colName, err := colHdlr.ColumnName("Bucket")
	if err != nil {
		return nil, err
	}

	rows, err := cer.conn.Query(fmt.Sprintf("SELECT %s FROM %s GROUP BY %s ORDER BY %s ASC", colName, entity.Table(), colName, colName))
	if err != nil {
		return nil, err
	}

	bucketNames := []string{}
	for rows.Next() {
		var bucket string
		if err := rows.Scan(&bucket); err != nil {
			return bucketNames, err
		}
		bucketNames = append(bucketNames, bucket)
	}

	return bucketNames, nil
}

func (cer *ConfigEntryRepository) DeleteBucket(bucket string) error {
	q, err := db.NewQuery(cer.conn, &ValueEntity{})
	if err != nil {
		return err
	}
	_, err = q.Delete().
		Where(map[string]interface{}{"Bucket": bucket}).
		Exec()
	return err
}

func (cer *ConfigEntryRepository) Close() error {
	return cer.conn.Close()
}
