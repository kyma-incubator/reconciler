package config

import (
	"bytes"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type cacheInvalidator struct {
	conn          db.Connection
	columnHandler *db.ColumnHandler
	selector      map[string]interface{}
	err           error
}

func newCacheInvalidator(conn db.Connection) (*cacheInvalidator, error) {
	columnHandler, err := db.NewColumnHandler(&CacheDependencyEntity{})
	if err != nil {
		return nil, err
	}
	return &cacheInvalidator{
		conn:          conn,
		columnHandler: columnHandler,
		selector:      make(map[string]interface{}),
	}, nil
}

func (ci *cacheInvalidator) WithBucket(bucket string) *cacheInvalidator {
	return ci.with("Bucket", bucket)
}

func (ci *cacheInvalidator) WithKey(key string) *cacheInvalidator {
	return ci.with("Key", key)
}

func (ci *cacheInvalidator) WithLabel(label string) *cacheInvalidator {
	return ci.with("Label", label)
}

func (ci *cacheInvalidator) WithCluster(cluster string) *cacheInvalidator {
	return ci.with("Cluster", cluster)
}

func (ci *cacheInvalidator) WithCacheID(cacheID int64) *cacheInvalidator {
	return ci.with("CacheID", cacheID)
}

func (ci *cacheInvalidator) with(colName string, colValue interface{}) *cacheInvalidator {
	colName, err := ci.columnHandler.ColumnName(colName)
	if err == nil {
		ci.selector[colName] = colValue
	} else {
		ci.err = err
	}
	return ci
}

func (ci *cacheInvalidator) Invalidate() error {
	//get cache dependencies
	depQuery, err := db.NewQuery(ci.conn, &CacheDependencyEntity{})
	if err != nil {
		return err
	}
	deps, err := depQuery.Select().Where(ci.selector).GetMany()
	if err != nil {
		return err
	}

	//get cache-entry IDs to invalidate
	cacheQuery, err := db.NewQuery(ci.conn, &CacheEntryEntity{})
	if err != nil {
		return err
	}
	_, err = cacheQuery.Delete().WhereIn("CacheID", ci.idsAsCSV(deps)).Exec()
	if err != nil {
		return err
	}
	return nil
}

func (ci *cacheInvalidator) idsAsCSV(deps []db.DatabaseEntity) string {
	deduplicate := make(map[int64]interface{}, len(deps))
	var buffer bytes.Buffer
	for _, dep := range deps {
		depEntity := dep.(*CacheDependencyEntity)
		if _, ok := deduplicate[depEntity.CacheID]; ok {
			continue
		}
		deduplicate[depEntity.CacheID] = nil //remember the cache IDs which are processed

		if buffer.Len() > 0 {
			buffer.WriteRune(',')
		}
		buffer.WriteByte(byte(depEntity.CacheID))
	}
	return buffer.String()
}
