package config

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/pkg/errors"
)

type KeyValueRepository struct {
	*Repository
}

func NewKeyValueRepository(dbFac db.ConnectionFactory, debug bool) (*KeyValueRepository, error) {
	repo, err := NewRepository(dbFac, debug)
	if err != nil {
		return nil, err
	}
	return &KeyValueRepository{repo}, nil
}

func (cer *KeyValueRepository) Keys() ([]*KeyEntity, error) {
	entity := &KeyEntity{}
	q, err := db.NewQuery(cer.conn, entity)
	if err != nil {
		return nil, err
	}

	//get fields used in sub-query
	colHdlr, err := db.NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}
	colNameVersion, err := colHdlr.ColumnName("Version")
	if err != nil {
		return nil, err
	}
	colNameKey, err := colHdlr.ColumnName("Key")
	if err != nil {
		return nil, err
	}

	//query all keys
	entities, err := q.Select().
		WhereIn("Version", fmt.Sprintf("SELECT MAX(%s) FROM %s GROUP BY %s",
			colNameVersion, entity.Table(), colNameKey)).
		OrderBy(map[string]string{"Key": "ASC"}).
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

func (cer *KeyValueRepository) KeyHistory(key string) ([]*KeyEntity, error) {
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

func (cer *KeyValueRepository) LatestKey(key string) (*KeyEntity, error) {
	q, err := db.NewQuery(cer.conn, &KeyEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Key": key}
	entity, err := q.Select().
		Where(whereCond).
		OrderBy(map[string]string{"Version": "DESC"}).
		Limit(1).
		GetOne()
	if err != nil {
		return nil, cer.handleNotFoundError(err, &KeyEntity{}, whereCond)
	}
	return entity.(*KeyEntity), nil
}

func (cer *KeyValueRepository) KeyByVersion(version int64) (*KeyEntity, error) {
	q, err := db.NewQuery(cer.conn, &KeyEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Version": version}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cer.handleNotFoundError(err, &KeyEntity{}, whereCond)
	}
	return entity.(*KeyEntity), nil
}

func (cer *KeyValueRepository) Key(key string, version int64) (*KeyEntity, error) {
	q, err := db.NewQuery(cer.conn, &KeyEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Key": key, "Version": version}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cer.handleNotFoundError(err, &KeyEntity{}, whereCond)
	}
	return entity.(*KeyEntity), nil
}

func (cer *KeyValueRepository) CreateKey(key *KeyEntity) (*KeyEntity, error) {
	q, err := db.NewQuery(cer.conn, key)
	if err != nil {
		return nil, err
	}
	existingKey, err := cer.LatestKey(key.Key)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if existingKey != nil && existingKey.Equal(key) {
		cer.logger.Debug(fmt.Sprintf("No differences found for key '%s': not creating new database entity", key.Key))
		return existingKey, nil
	}
	return key, q.Insert().Exec()
}

func (cer *KeyValueRepository) DeleteKey(key string) error {
	//bundle DB operations
	dbOps := func() error {
		if err := cer.deleteValuesByKey(key); err != nil {
			return err
		}

		//delete all values mapped to the key
		qKey, err := db.NewQuery(cer.conn, &KeyEntity{})
		if err != nil {
			return err
		}
		_, err = qKey.Delete().
			Where(map[string]interface{}{"Key": key}).
			Exec()
		return err
	}

	//run db-operations transactional
	cer.logger.Debug("Begin transactional DB context")
	tx, err := cer.conn.Begin()
	if err != nil {
		return err
	}
	if err := dbOps(); err != nil {
		cer.logger.Info("Rollback transactional DB context")
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", tx.Rollback()))
		}
		return err
	}
	cer.logger.Debug("Commit transactional DB context")
	return tx.Commit()
}

func (cer *KeyValueRepository) ValuesByBucket(bucket string) ([]*ValueEntity, error) {
	entity := &ValueEntity{}
	q, err := db.NewQuery(cer.conn, entity)
	if err != nil {
		return nil, err
	}

	//get fields used in sub-query
	colHdlr, err := db.NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}
	colVersion, err := colHdlr.ColumnName("Version")
	if err != nil {
		return nil, err
	}
	colKey, err := colHdlr.ColumnName("Key")
	if err != nil {
		return nil, err
	}
	colBucket, err := colHdlr.ColumnName("Bucket")
	if err != nil {
		return nil, err
	}

	//query all values in bucket (return only the latest value-entry per key)
	entities, err := q.Select().
		WhereIn("Version", fmt.Sprintf("SELECT MAX(%s) FROM %s WHERE %s=$1 GROUP BY %s",
			colVersion, entity.Table(), colBucket, colKey), bucket).
		OrderBy(map[string]string{"Key": "ASC"}).
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

func (cer *KeyValueRepository) ValuesByKey(key *KeyEntity) ([]*ValueEntity, error) {
	entity := &ValueEntity{}
	q, err := db.NewQuery(cer.conn, entity)
	if err != nil {
		return nil, err
	}

	//get fields used in sub-query
	colHdlr, err := db.NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}
	colBucket, err := colHdlr.ColumnName("Bucket")
	if err != nil {
		return nil, err
	}
	colVersion, err := colHdlr.ColumnName("Version")
	if err != nil {
		return nil, err
	}
	colKey, err := colHdlr.ColumnName("Key")
	if err != nil {
		return nil, err
	}
	colKeyVersion, err := colHdlr.ColumnName("KeyVersion")
	if err != nil {
		return nil, err
	}

	//query all values in bucket (return only the latest value-entry per key)
	entities, err := q.Select().
		WhereIn("Version", fmt.Sprintf("SELECT MAX(%s) FROM %s WHERE %s=$1 AND %s=$2 GROUP BY %s, %s",
			colVersion, entity.Table(), colKey, colKeyVersion, colKey, colBucket), key.Key, key.Version).
		OrderBy(map[string]string{"Bucket": "ASC"}).
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

func (cer *KeyValueRepository) ValueHistory(bucket, key string) ([]*ValueEntity, error) {
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

func (cer *KeyValueRepository) LatestValue(bucket, key string) (*ValueEntity, error) {
	q, err := db.NewQuery(cer.conn, &ValueEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Key": key, "Bucket": bucket}
	entity, err := q.Select().
		Where(whereCond).
		OrderBy(map[string]string{"Version": "DESC"}).
		Limit(1).
		GetOne()
	if err != nil {
		return nil, cer.handleNotFoundError(err, &ValueEntity{}, whereCond)
	}
	return entity.(*ValueEntity), nil
}

func (cer *KeyValueRepository) Value(bucket, key string, version int64) (*ValueEntity, error) {
	q, err := db.NewQuery(cer.conn, &ValueEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Bucket": bucket, "Key": key, "Version": version}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cer.handleNotFoundError(err, &ValueEntity{}, whereCond)
	}
	return entity.(*ValueEntity), nil
}

func (cer *KeyValueRepository) CreateValue(value *ValueEntity) (*ValueEntity, error) {
	existingValue, err := cer.LatestValue(value.Bucket, value.Key)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if existingValue != nil && existingValue.Equal(value) {
		cer.logger.Debug(fmt.Sprintf("No differences found for value of key '%s': not creating new database entity", value.Key))
		return existingValue, nil
	}

	//insert operation
	dbOps := func() (*ValueEntity, error) {

		//add value entity
		q, err := db.NewQuery(cer.conn, value)
		if err != nil {
			return nil, err
		}
		valueEntity, err := value, q.Insert().Exec()
		if err != nil {
			return valueEntity, err
		}

		//compare data type of value-entity with value type of key-entity
		keyEntity, err := cer.Key(valueEntity.Key, valueEntity.KeyVersion)
		if err != nil {
			return valueEntity, errors.Wrap(err, fmt.Sprintf("Failed to retrieve key entity for value entity '%s'", valueEntity))
		}
		if valueEntity.DataType != keyEntity.DataType {
			return valueEntity, fmt.Errorf("Data type of value entity (%s) is different to data type of key entity (%s)",
				valueEntity.DataType, keyEntity.DataType)
		}

		//TODO: INVALIDATE all cache entries which have the existing value entry as dependency!

		//done
		return valueEntity, nil
	}

	//run db-operation transactional
	cer.logger.Debug("Begin transactional DB context")
	tx, err := cer.conn.Begin()
	if err != nil {
		return nil, err
	}
	valueEntity, err := dbOps()
	if err != nil {
		cer.logger.Info("Rollback transactional DB context")
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", rollbackErr))
		}
		return nil, err
	}
	cer.logger.Debug("Commit transactional DB context")
	return valueEntity, tx.Commit()
}

func (cer *KeyValueRepository) deleteValuesByKey(key string) error {
	q, err := db.NewQuery(cer.conn, &ValueEntity{})
	if err != nil {
		return err
	}

	//TODO: INVALIDATE all cache entries which have these value entries as dependency!

	_, err = q.Delete().
		Where(map[string]interface{}{"Key": key}).
		Exec()
	return err
}

func (cer *KeyValueRepository) Buckets() ([]*BucketEntity, error) {
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
		whereCond := map[string]interface{}{"Bucket": bucketName}
		entity, err := q.Select().
			Where(whereCond).
			OrderBy(map[string]string{"Created": "ASC"}).
			Limit(1).
			GetOne()
		if err != nil {
			return nil, cer.handleNotFoundError(err, &BucketEntity{}, whereCond)
		}
		buckets = append(buckets, entity.(*BucketEntity))
	}

	return buckets, nil
}

func (cer *KeyValueRepository) bucketNames() ([]string, error) {
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

func (cer *KeyValueRepository) DeleteBucket(bucket string) error {
	q, err := db.NewQuery(cer.conn, &BucketEntity{})
	if err != nil {
		return err
	}

	//TODO: INVALIDATE all cache entries which have these value entries as dependency!

	_, err = q.Delete().
		Where(map[string]interface{}{"Bucket": bucket}).
		Exec()
	return err
}

func (cer *KeyValueRepository) Close() error {
	return cer.conn.Close()
}
