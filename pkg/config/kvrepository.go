package config

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
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

	return cer.transactional(dbOps)
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

	//validate value with metadata defined in key before storing it
	key, err := cer.Key(value.Key, value.KeyVersion)
	if err != nil {
		return nil, err //provided key doesn't exist
	}

	if value.DataType == "" { //verify data-type is properly defined
		value.DataType = key.DataType
	} else if value.DataType != key.DataType {
		return nil, fmt.Errorf("Configured data type '%s' is not allowed: key defines the data type '%s'",
			value.DataType, key.DataType)
	}

	if err := key.Validate(value.Value); err != nil {
		return nil, err //provided value is invalid
	}

	//insert operation
	dbOps := func() (interface{}, error) {
		//add value entity
		q, err := db.NewQuery(cer.conn, value)
		if err != nil {
			return nil, err
		}
		valueEntity, err := value, q.Insert().Exec()
		if err != nil {
			return valueEntity, err
		}

		//new value provided - invalidate caches which were using the old value
		if err := cer.cache.Invalidate().WithBucket(value.Bucket).WithKey(value.Key).Exec(false); err != nil {
			return valueEntity, err
		}

		//done
		return valueEntity, nil
	}

	result, err := cer.transactionalResult(dbOps)
	var valueEntity *ValueEntity
	if result != nil {
		valueEntity = result.(*ValueEntity)
	}

	return valueEntity, err
}

func (cer *KeyValueRepository) deleteValuesByKey(key string) error {
	dbOps := func() error {
		q, err := db.NewQuery(cer.conn, &ValueEntity{})
		if err != nil {
			return err
		}

		if err := cer.cache.Invalidate().WithKey(key).Exec(false); err != nil {
			return err
		}

		_, err = q.Delete().
			Where(map[string]interface{}{"Key": key}).
			Exec()
		return err
	}
	return cer.transactional(dbOps)
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
	dbOps := func() error {
		q, err := db.NewQuery(cer.conn, &BucketEntity{})
		if err != nil {
			return err
		}

		if err := cer.cache.Invalidate().WithBucket(bucket).Exec(false); err != nil {
			return err
		}

		_, err = q.Delete().
			Where(map[string]interface{}{"Bucket": bucket}).
			Exec()
		return err
	}
	return cer.transactional(dbOps)
}

func (cer *KeyValueRepository) Close() error {
	return cer.conn.Close()
}
