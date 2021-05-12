package config

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type EntityNotFoundError struct {
	entity     db.DatabaseEntity
	identifier map[string]interface{}
}

func (e *EntityNotFoundError) Error() string {
	var idents bytes.Buffer
	if e.identifier != nil {
		for k, v := range e.identifier {
			if idents.Len() > 0 {
				idents.WriteRune(',')
			}
			idents.WriteString(fmt.Sprintf("%s=%v", k, v))
		}
	}
	return fmt.Sprintf("Entity of type '%T' with identifier '%v' not found", e.entity, idents.String())
}

type EntryRepository struct {
	conn   db.Connection
	logger *zap.Logger
}

func NewEntryRepository(dbFac db.ConnectionFactory, debug bool) (*EntryRepository, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	conn, err := dbFac.NewConnection()
	return &EntryRepository{
		conn:   conn,
		logger: logger,
	}, err
}

func IsNotFoundError(err error) bool {
	return reflect.TypeOf(err) == reflect.TypeOf(&EntityNotFoundError{})
}

func (cer *EntryRepository) handleNotFoundError(err error, entity db.DatabaseEntity,
	identifier map[string]interface{}) error {
	if err == sql.ErrNoRows {
		return &EntityNotFoundError{
			entity:     entity,
			identifier: identifier,
		}
	}
	return err
}

func (cer *EntryRepository) Keys() ([]*KeyEntity, error) {
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

func (cer *EntryRepository) KeyHistory(key string) ([]*KeyEntity, error) {
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

func (cer *EntryRepository) LatestKey(key string) (*KeyEntity, error) {
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

func (cer *EntryRepository) KeyByVersion(version int64) (*KeyEntity, error) {
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

func (cer *EntryRepository) Key(key string, version int64) (*KeyEntity, error) {
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

func (cer *EntryRepository) CreateKey(key *KeyEntity) (*KeyEntity, error) {
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

func (cer *EntryRepository) DeleteKey(key string) error {
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
		cer.logger.Debug("Rollback transactional DB context")
		return errors.Wrap(err, fmt.Sprintf("Rollback of db operations failed: %s", tx.Rollback()))
	}
	cer.logger.Debug("Commit transactional DB context")
	return tx.Commit()
}

func (cer *EntryRepository) ValuesByBucket(bucket *BucketEntity) ([]*ValueEntity, error) {
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
			colVersion, entity.Table(), colBucket, colKey), bucket.Bucket).
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

func (cer *EntryRepository) ValuesByKey(key *KeyEntity) ([]*ValueEntity, error) {
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

func (cer *EntryRepository) ValueHistory(bucket, key string) ([]*ValueEntity, error) {
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

func (cer *EntryRepository) LatestValue(bucket, key string) (*ValueEntity, error) {
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

func (cer *EntryRepository) Value(bucket, key string, version int64) (*ValueEntity, error) {
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

func (cer *EntryRepository) CreateValue(value *ValueEntity) (*ValueEntity, error) {
	q, err := db.NewQuery(cer.conn, value)
	if err != nil {
		return nil, err
	}
	existingValue, err := cer.LatestValue(value.Bucket, value.Key)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if existingValue != nil && existingValue.Equal(value) {
		cer.logger.Debug(fmt.Sprintf("No differences found for value of key '%s': not creating new database entity", value.Key))
		return existingValue, nil
	}
	return value, q.Insert().Exec()
}

func (cer *EntryRepository) deleteValuesByKey(key string) error {
	q, err := db.NewQuery(cer.conn, &ValueEntity{})
	if err != nil {
		return err
	}
	_, err = q.Delete().
		Where(map[string]interface{}{"Key": key}).
		Exec()
	return err
}

func (cer *EntryRepository) Buckets() ([]*BucketEntity, error) {
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

func (cer *EntryRepository) bucketNames() ([]string, error) {
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

func (cer *EntryRepository) DeleteBucket(bucket string) error {
	q, err := db.NewQuery(cer.conn, &BucketEntity{})
	if err != nil {
		return err
	}
	_, err = q.Delete().
		Where(map[string]interface{}{"Bucket": bucket}).
		Exec()
	return err
}

func (cer *EntryRepository) Close() error {
	return cer.conn.Close()
}
