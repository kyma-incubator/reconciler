package repository

import (
	"bytes"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"go.uber.org/zap"
)

type cacheDependencyManager struct {
	conn   db.Connection
	logger *zap.SugaredLogger
}

type record struct {
	*cacheDependencyManager
	cacheEntry *model.CacheEntryEntity
	cacheDeps  []*model.ValueEntity
}

type invalidate struct {
	*cacheDependencyManager
	selector map[string]interface{}
}

type get struct {
	*cacheDependencyManager
	selector map[string]interface{}
}

func newCacheDependencyManager(conn db.Connection, debug bool) *cacheDependencyManager {
	return &cacheDependencyManager{
		conn:   conn,
		logger: logger.NewLogger(debug),
	}
}

func (cdm *cacheDependencyManager) transactional(desc string, dbOps func() error) error {
	if err := db.Transaction(cdm.conn, dbOps, cdm.logger); err != nil {
		return fmt.Errorf("failed to execute database transaction '%s': %s", desc, err)
	}
	return nil
}

func (cdm *cacheDependencyManager) Record(cacheEntry *model.CacheEntryEntity, cacheDeps []*model.ValueEntity) *record {
	return &record{
		cdm,
		cacheEntry,
		cacheDeps,
	}
}

func (r *record) Exec(newTx bool) error {
	if r.cacheEntry.ID <= 0 {
		return fmt.Errorf("cache entry '%s' has no ID: indicates that cache entity is not persisted in database", r.cacheEntry)
	}
	dbOps := func() error {
		//track deps in DB
		for _, value := range r.cacheDeps {
			q, err := db.NewQuery(r.conn, &model.CacheDependencyEntity{
				Bucket:    value.Bucket,
				Key:       value.Key,
				Label:     r.cacheEntry.Label,
				RuntimeID: r.cacheEntry.RuntimeID,
				CacheID:   r.cacheEntry.ID,
			}, r.logger)
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

func (i *invalidate) WithRuntimeID(runtimeID string) *invalidate {
	return i.with("RuntimeID", runtimeID)
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
		depQuery, err := db.NewQuery(i.conn, &model.CacheDependencyEntity{}, i.logger)
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
		i.logger.Debugf("Found %d cache dependencies for selector '%v'", len(deps), i.selector)

		if len(deps) == 0 { //not deps found - nothing to invalidate
			return nil
		}

		//get cache-entry IDs to invalidate
		cacheEntityIdsCSV, cntUniqueIds := i.cacheIDsCSV(deps)
		i.logger.Debugf("Identified %d cache entities which match selector '%v': %s", cntUniqueIds, i.selector, cacheEntityIdsCSV)

		//drop all cache entities
		cacheQuery, err := db.NewQuery(i.conn, &model.CacheEntryEntity{}, i.logger)
		if err != nil {
			return err
		}
		deletedEntries, err := cacheQuery.Delete().WhereIn("ID", cacheEntityIdsCSV).Exec()
		if err != nil {
			return err
		}
		i.logger.Debugf("Deleted %d cache entries matching selector '%v'", deletedEntries, i.selector)

		//drop all cache dependencies of the dropped cache entities
		cacheDepQuery, err := db.NewQuery(i.conn, &model.CacheDependencyEntity{}, i.logger)
		if err != nil {
			return err
		}
		deletedDeps, err := cacheDepQuery.Delete().WhereIn("CacheID", cacheEntityIdsCSV).Exec()
		if err != nil {
			return err
		}
		i.logger.Debugf("Deleted %d cache dependencies matching selector '%v'", deletedDeps, i.selector)

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
		depEntity := dep.(*model.CacheDependencyEntity)
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

func (c *get) WithRuntimeID(runtimeID string) *get {
	return c.with("RuntimeID", runtimeID)
}

func (c *get) WithCacheID(cacheID int64) *get {
	return c.with("CacheID", cacheID)
}

func (c *get) with(colName string, colValue interface{}) *get {
	c.selector[colName] = colValue
	return c
}

func (c *get) Exec() ([]*model.CacheDependencyEntity, error) {
	cntQuery, err := db.NewQuery(c.conn, &model.CacheDependencyEntity{}, c.logger)
	if err != nil {
		return nil, err
	}
	deps, err := cntQuery.Select().Where(c.selector).GetMany()
	if err != nil {
		return nil, err
	}
	var result []*model.CacheDependencyEntity
	for _, dep := range deps {
		result = append(result, dep.(*model.CacheDependencyEntity))
	}
	return result, nil
}
