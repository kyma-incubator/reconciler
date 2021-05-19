package config

import (
	"bytes"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type cacheDependencyManager struct {
	conn   db.Connection
	logger *zap.Logger
}

type record struct {
	*cacheDependencyManager
	cacheEntry *CacheEntryEntity
	cacheDeps  []*ValueEntity
}

type invalidate struct {
	*cacheDependencyManager
	selector map[string]interface{}
}

type get struct {
	*cacheDependencyManager
	selector map[string]interface{}
}

func newCacheDependencyManager(conn db.Connection, debug bool) (*cacheDependencyManager, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &cacheDependencyManager{
		conn:   conn,
		logger: logger,
	}, nil
}

func (cdm *cacheDependencyManager) transactional(desc string, dbOps func() error) error {
	cdm.logger.Debug("Beginning DB transaction")
	tx, err := cdm.conn.Begin()
	if err != nil {
		return err
	}
	if err := dbOps(); err != nil {
		cdm.logger.Debug("Rollback of DB transaction")
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Wrap(err, fmt.Sprintf("Rollback of database transaction '%s' failed: %s", desc, rollbackErr))
		}
		return err
	}
	cdm.logger.Debug("Committing DB transaction")
	return tx.Commit()
}

func (cdm *cacheDependencyManager) Record(cacheEntry *CacheEntryEntity, cacheDeps []*ValueEntity) *record {
	return &record{
		cdm,
		cacheEntry,
		cacheDeps,
	}
}

func (r *record) Exec(newTx bool) error {
	dbOps := func() error {
		//track deps in DB
		for _, value := range r.cacheDeps {
			q, err := db.NewQuery(r.conn, &CacheDependencyEntity{
				Bucket:  value.Bucket,
				Key:     value.Key,
				Label:   r.cacheEntry.Label,
				Cluster: r.cacheEntry.Cluster,
				CacheID: r.cacheEntry.ID,
			})
			if err != nil {
				return err
			}
			if err := q.Insert().Exec(); err != nil {
				return err
			}
		}
		return nil
	}

	if newTx { //start new DB transaction
		return r.transactional("recording cache dependencies", dbOps)
	}
	return dbOps() //no new DB transaction requested
}

func (cdm *cacheDependencyManager) Invalidate() *invalidate {
	return &invalidate{
		cdm,
		make(map[string]interface{}),
	}
}

func (i *invalidate) WithBucket(bucket string) *invalidate {
	return i.with("Bucket", bucket)
}

func (i *invalidate) WithKey(key string) *invalidate {
	return i.with("Key", key)
}

func (i *invalidate) WithLabel(label string) *invalidate {
	return i.with("Label", label)
}

func (i *invalidate) WithCluster(cluster string) *invalidate {
	return i.with("Cluster", cluster)
}

func (i *invalidate) WithCacheID(cacheID int64) *invalidate {
	return i.with("CacheID", cacheID)
}

func (i *invalidate) with(colName string, colValue interface{}) *invalidate {
	i.selector[colName] = colValue
	return i
}

func (i *invalidate) Exec(newTx bool) error {
	dbOps := func() error {
		//get cache dependencies
		depQuery, err := db.NewQuery(i.conn, &CacheDependencyEntity{})
		if err != nil {
			return err
		}

		if len(i.selector) == 0 {
			i.logger.Info("No cache-dependency selector defined: this will invalidate all cache entries")
		}

		deps, err := depQuery.Select().Where(i.selector).GetMany()
		if err != nil {
			return err
		}
		i.logger.Debug(fmt.Sprintf("Found %d cache dependencies for selector '%v'", len(deps), i.selector))

		//get cache-entry IDs to invalidate
		cacheEntityIdsCSV, uniqueIds := i.cacheIDsCSV(deps)
		i.logger.Debug(fmt.Sprintf("Identified %d cache entities which match selector '%v': %s", uniqueIds, i.selector, cacheEntityIdsCSV))

		//drop all cache entities
		cacheQuery, err := db.NewQuery(i.conn, &CacheEntryEntity{})
		if err != nil {
			return err
		}
		deletedEntries, err := cacheQuery.Delete().WhereIn("ID", cacheEntityIdsCSV).Exec()
		if err != nil {
			return err
		}
		i.logger.Debug(fmt.Sprintf("Deleted %d cache entries matching selector '%v'", deletedEntries, i.selector))

		//drop all cache dependencies of the dropped cache entities
		cacheDepQuery, err := db.NewQuery(i.conn, &CacheDependencyEntity{})
		if err != nil {
			return err
		}
		deletedDeps, err := cacheDepQuery.Delete().WhereIn("CacheID", cacheEntityIdsCSV).Exec()
		if err != nil {
			return err
		}
		i.logger.Debug(fmt.Sprintf("Deleted %d cache dependencies matching selector '%v'", deletedDeps, i.selector))

		return nil
	}

	if newTx { //start new DB transaction
		return i.transactional("invalidating cache entries", dbOps)
	}
	return dbOps() //no new DB transaction requested
}

func (i *invalidate) cacheIDsCSV(deps []db.DatabaseEntity) (string, int) {
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
		buffer.WriteString(fmt.Sprintf("%d", depEntity.CacheID))
	}
	return buffer.String(), len(deduplicate)
}

func (cdm *cacheDependencyManager) Get() *get {
	return &get{
		cdm,
		make(map[string]interface{}),
	}
}

func (c *get) WithBucket(bucket string) *get {
	return c.with("Bucket", bucket)
}

func (c *get) WithKey(key string) *get {
	return c.with("Key", key)
}

func (c *get) WithLabel(label string) *get {
	return c.with("Label", label)
}

func (c *get) WithCluster(cluster string) *get {
	return c.with("Cluster", cluster)
}

func (c *get) WithCacheID(cacheID int64) *get {
	return c.with("CacheID", cacheID)
}

func (c *get) with(colName string, colValue interface{}) *get {
	c.selector[colName] = colValue
	return c
}

func (c *get) Exec() ([]*CacheDependencyEntity, error) {
	cntQuery, err := db.NewQuery(c.conn, &CacheDependencyEntity{})
	if err != nil {
		return nil, err
	}
	deps, err := cntQuery.Select().Where(c.selector).GetMany()
	if err != nil {
		return nil, err
	}
	result := []*CacheDependencyEntity{}
	for _, dep := range deps {
		result = append(result, dep.(*CacheDependencyEntity))
	}
	return result, nil
}
