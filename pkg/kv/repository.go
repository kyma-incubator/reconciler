package kv

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
)

type Repository struct {
	*repository.Repository
}

func NewRepository(conn db.Connection, debug bool) (*Repository, error) {
	repo, err := repository.NewRepository(conn, debug)
	if err != nil {
		return nil, err
	}
	return &Repository{repo}, nil
}

func (cer *Repository) Keys() ([]*model.KeyEntity, error) {
	entity := &model.KeyEntity{}
	q, err := db.NewQuery(cer.Conn, entity, cer.Logger)
	if err != nil {
		return nil, err
	}

	//get fields used in sub-query
	colHdlr, err := db.NewColumnHandler(entity, cer.Conn, cer.Logger)
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
	var result []*model.KeyEntity
	for _, entity := range entities {
		result = append(result, entity.(*model.KeyEntity))
	}
	return result, nil
}

func (cer *Repository) KeyHistory(key string) ([]*model.KeyEntity, error) {
	entity := &model.KeyEntity{}
	q, err := db.NewQuery(cer.Conn, entity, cer.Logger)
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
	var result []*model.KeyEntity
	for _, entity := range entities {
		result = append(result, entity.(*model.KeyEntity))
	}
	return result, nil
}

func (cer *Repository) LatestKey(key string) (*model.KeyEntity, error) {
	q, err := db.NewQuery(cer.Conn, &model.KeyEntity{}, cer.Logger)
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
		return nil, cer.NewNotFoundError(err, &model.KeyEntity{}, whereCond)
	}
	return entity.(*model.KeyEntity), nil
}

func (cer *Repository) KeyByVersion(version int64) (*model.KeyEntity, error) {
	q, err := db.NewQuery(cer.Conn, &model.KeyEntity{}, cer.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Version": version}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cer.NewNotFoundError(err, &model.KeyEntity{}, whereCond)
	}
	return entity.(*model.KeyEntity), nil
}

func (cer *Repository) Key(key string, version int64) (*model.KeyEntity, error) {
	q, err := db.NewQuery(cer.Conn, &model.KeyEntity{}, cer.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Key": key, "Version": version}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cer.NewNotFoundError(err, &model.KeyEntity{}, whereCond)
	}
	return entity.(*model.KeyEntity), nil
}

func (cer *Repository) CreateKey(key *model.KeyEntity) (*model.KeyEntity, error) {
	q, err := db.NewQuery(cer.Conn, key, cer.Logger)
	if err != nil {
		return nil, err
	}
	existingKey, err := cer.LatestKey(key.Key)
	if err != nil && !repository.IsNotFoundError(err) {
		return nil, err
	}
	if existingKey != nil && existingKey.Equal(key) {
		cer.Logger.Debugf("No differences found for key '%s': not creating new database entity", key.Key)
		return existingKey, nil
	}
	return key, q.Insert().Exec()
}

func (cer *Repository) DeleteKey(key string) error {
	//bundle DB operations
	dbOps := func() error {
		//delete all cache entities which were using a value of this key
		if err := cer.CacheDep.Invalidate().WithKey(key).Exec(false); err != nil {
			return err
		}

		//delete the values mapped to this key
		q, err := db.NewQuery(cer.Conn, &model.ValueEntity{}, cer.Logger)
		if err != nil {
			return err
		}
		_, err = q.Delete().
			Where(map[string]interface{}{"Key": key}).
			Exec()
		if err != nil {
			return err
		}

		//delete the key
		qKey, err := db.NewQuery(cer.Conn, &model.KeyEntity{}, cer.Logger)
		if err != nil {
			return err
		}
		_, err = qKey.Delete().
			Where(map[string]interface{}{"Key": key}).
			Exec()
		return err
	}

	return cer.Transactional(dbOps)
}

func (cer *Repository) ValuesByBucket(bucket string) ([]*model.ValueEntity, error) {
	entity := &model.ValueEntity{}
	q, err := db.NewQuery(cer.Conn, entity, cer.Logger)
	if err != nil {
		return nil, err
	}

	//get fields used in sub-query
	colHdlr, err := db.NewColumnHandler(entity, cer.Conn, cer.Logger)
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
	var result []*model.ValueEntity
	for _, entity := range entities {
		result = append(result, entity.(*model.ValueEntity))
	}
	return result, nil
}

func (cer *Repository) ValuesByKey(key *model.KeyEntity) ([]*model.ValueEntity, error) {
	entity := &model.ValueEntity{}
	q, err := db.NewQuery(cer.Conn, entity, cer.Logger)
	if err != nil {
		return nil, err
	}

	//get fields used in sub-query
	colHdlr, err := db.NewColumnHandler(entity, cer.Conn, cer.Logger)
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
	var result []*model.ValueEntity
	for _, entity := range entities {
		result = append(result, entity.(*model.ValueEntity))
	}
	return result, nil
}

func (cer *Repository) ValueHistory(bucket, key string) ([]*model.ValueEntity, error) {
	entity := &model.ValueEntity{}
	q, err := db.NewQuery(cer.Conn, entity, cer.Logger)
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
	var result []*model.ValueEntity
	for _, entity := range entities {
		result = append(result, entity.(*model.ValueEntity))
	}
	return result, nil
}

func (cer *Repository) LatestValue(bucket, key string) (*model.ValueEntity, error) {
	q, err := db.NewQuery(cer.Conn, &model.ValueEntity{}, cer.Logger)
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
		return nil, cer.NewNotFoundError(err, &model.ValueEntity{}, whereCond)
	}
	return entity.(*model.ValueEntity), nil
}

func (cer *Repository) Value(bucket, key string, version int64) (*model.ValueEntity, error) {
	q, err := db.NewQuery(cer.Conn, &model.ValueEntity{}, cer.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Bucket": bucket, "Key": key, "Version": version}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cer.NewNotFoundError(err, &model.ValueEntity{}, whereCond)
	}
	return entity.(*model.ValueEntity), nil
}

func (cer *Repository) CreateValue(value *model.ValueEntity) (*model.ValueEntity, error) {
	existingValue, err := cer.LatestValue(value.Bucket, value.Key)
	if err != nil && !repository.IsNotFoundError(err) {
		return nil, err
	}
	if existingValue != nil && existingValue.Equal(value) {
		cer.Logger.Debugf("No differences found for value of key '%s': not creating new database entity", value.Key)
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
		return nil, &InvalidDataTypeError{
			Key:             key,
			InvalidDataType: value.DataType,
		}
	}

	if err := key.Validate(value.Value); err != nil {
		return nil, err //provided value is invalid
	}

	//insert operation
	dbOps := func() (interface{}, error) {
		//add value entity
		q, err := db.NewQuery(cer.Conn, value, cer.Logger)
		if err != nil {
			return nil, err
		}
		valueEntity, err := value, q.Insert().Exec()
		if err != nil {
			return valueEntity, err
		}

		//new value provided - invalidate caches which were using the old value
		if err := cer.CacheDep.Invalidate().WithBucket(value.Bucket).WithKey(value.Key).Exec(false); err != nil {
			return valueEntity, err
		}

		//done
		return valueEntity, nil
	}

	result, err := cer.TransactionalResult(dbOps)
	var valueEntity *model.ValueEntity
	if result != nil {
		valueEntity = result.(*model.ValueEntity)
	}

	return valueEntity, err
}

func (cer *Repository) DeleteValue(key, bucket string) error {
	//bundle DB operations
	dbOps := func() error {
		//delete all cache entities which were using a value of this key in this bucket
		if err := cer.CacheDep.Invalidate().WithKey(key).WithBucket(bucket).Exec(false); err != nil {
			return err
		}

		//delete the values mapped to this key in this bucket
		q, err := db.NewQuery(cer.Conn, &model.ValueEntity{}, cer.Logger)
		if err != nil {
			return err
		}
		_, err = q.Delete().
			Where(map[string]interface{}{"Key": key, "Bucket": bucket}).
			Exec()
		return err
	}

	return cer.Transactional(dbOps)
}

func (cer *Repository) Buckets() ([]*model.BucketEntity, error) {
	bucketNames, err := cer.bucketNames()
	if err != nil {
		return nil, err
	}

	var buckets []*model.BucketEntity
	for _, bucketName := range bucketNames {
		q, err := db.NewQuery(cer.Conn, &model.BucketEntity{}, cer.Logger)
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
			return nil, cer.NewNotFoundError(err, &model.BucketEntity{}, whereCond)
		}
		buckets = append(buckets, entity.(*model.BucketEntity))
	}

	return buckets, nil
}

func (cer *Repository) bucketNames() ([]string, error) {
	entity := &model.BucketEntity{}

	colHdlr, err := db.NewColumnHandler(entity, cer.Conn, cer.Logger)
	if err != nil {
		return nil, err
	}

	colName, err := colHdlr.ColumnName("Bucket")
	if err != nil {
		return nil, err
	}

	rows, err := cer.Conn.Query(fmt.Sprintf("SELECT %s FROM %s GROUP BY %s ORDER BY %s ASC", colName, entity.Table(), colName, colName))
	if err != nil {
		return nil, err
	}

	var bucketNames []string
	for rows.Next() {
		var bucket string
		if err := rows.Scan(&bucket); err != nil {
			return bucketNames, err
		}
		bucketNames = append(bucketNames, bucket)
	}

	return bucketNames, nil
}

func (cer *Repository) DeleteBucket(bucket string) error {
	dbOps := func() error {
		//invalidate all cache entities which were using values from this bucket
		if err := cer.CacheDep.Invalidate().WithBucket(bucket).Exec(false); err != nil {
			return err
		}

		//delete the bucket
		q, err := db.NewQuery(cer.Conn, &model.BucketEntity{}, cer.Logger)
		if err != nil {
			return err
		}
		_, err = q.Delete().
			Where(map[string]interface{}{"Bucket": bucket}).
			Exec()
		return err
	}
	return cer.Transactional(dbOps)
}

type InvalidDataTypeError struct {
	Key             *model.KeyEntity
	InvalidDataType model.DataType
}

func (e *InvalidDataTypeError) Error() string {
	return fmt.Sprintf("Configured data type '%s' is not allowed: key '%s' defines the data type '%s'",
		e.InvalidDataType, e.Key, e.Key.DataType)
}

func IsInvalidDataTypeError(err error) bool {
	_, ok := err.(*InvalidDataTypeError)
	return ok
}
