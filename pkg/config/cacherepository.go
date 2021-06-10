package config

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type CacheRepository struct {
	*Repository
}

func NewCacheRepository(dbFac db.ConnectionFactory, debug bool) (*CacheRepository, error) {
	repo, err := NewRepository(dbFac, debug)
	if err != nil {
		return nil, err
	}
	return &CacheRepository{repo}, nil
}

func (cr *CacheRepository) All() ([]*CacheEntryEntity, error) {
	q, err := db.NewQuery(cr.conn, &CacheEntryEntity{})
	if err != nil {
		return nil, err
	}
	entities, err := q.Select().
		GetMany()
	if err != nil {
		return nil, err
	}
	result := []*CacheEntryEntity{}
	for _, entity := range entities {
		result = append(result, entity.(*CacheEntryEntity))
	}
	return result, nil
}

func (cr *CacheRepository) Get(label, cluster string) (*CacheEntryEntity, error) {
	q, err := db.NewQuery(cr.conn, &CacheEntryEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Label": label, "Cluster": cluster}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cr.handleNotFoundError(err, &CacheEntryEntity{}, whereCond)
	}
	return entity.(*CacheEntryEntity), nil
}

func (cr *CacheRepository) GetByID(id int64) (*CacheEntryEntity, error) {
	q, err := db.NewQuery(cr.conn, &CacheEntryEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"ID": id}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cr.handleNotFoundError(err, &CacheEntryEntity{}, whereCond)
	}
	return entity.(*CacheEntryEntity), nil
}

func (cr *CacheRepository) Add(cacheEntry *CacheEntryEntity, cacheDeps []*ValueEntity) (*CacheEntryEntity, error) {
	//Get exiting cache entry
	inCache, err := cr.Get(cacheEntry.Label, cacheEntry.Cluster)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if inCache != nil {
		//check if the existing cache entry is still valid otherwise invalidate it
		if inCache.Equal(cacheEntry) {
			cr.logger.Debug(fmt.Sprintf("No differences found for cache entry '%s' (cluster '%s'): not creating new database entity",
				cacheEntry.Label, cacheEntry.Cluster))
			return inCache, nil
		}
		cr.logger.Debug(fmt.Sprintf("Existing cache entry '%s' is outdated and will be invalidated", inCache))
		if err := cr.InvalidateByID(inCache.ID); err != nil {
			return cacheEntry, err
		}
	}

	//create new cache entry and track its dependencies
	dbOps := func() (interface{}, error) {
		q, err := db.NewQuery(cr.conn, cacheEntry)
		if err != nil {
			return cacheEntry, err
		}
		if err := q.Insert().Exec(); err != nil {
			return cacheEntry, err
		}
		if err := cr.cacheDep.Record(cacheEntry, cacheDeps).Exec(false); err != nil {
			return cacheEntry, err
		}
		return cacheEntry, err
	}

	var cacheEntryEntity *CacheEntryEntity
	result, err := cr.transactionalResult(dbOps)
	if result != nil {
		cacheEntryEntity = result.(*CacheEntryEntity)
	}
	return cacheEntryEntity, err
}

func (cr *CacheRepository) Invalidate(label, cluster string) error {
	dbOps := func() error {
		//invalidate the cache entity and drop all tracked dependencies
		if err := cr.cacheDep.Invalidate().WithLabel(label).WithCluster(cluster).Exec(false); err != nil {
			return err
		}

		//as cache dependencies are optional we cannot rely that the previous
		//invalidation dropped the cache entity: delete the entity also explicitly
		q, err := db.NewQuery(cr.conn, &CacheEntryEntity{})
		if err != nil {
			return err
		}
		deleted, err := q.Delete().
			Where(map[string]interface{}{"Label": label, "Cluster": cluster}).
			Exec()
		cr.logger.Debug(fmt.Sprintf("Deleted %d cache entries which had no dependencies", deleted))
		return err
	}
	return cr.transactional(dbOps)
}

func (cr *CacheRepository) InvalidateByID(id int64) error {
	dbOps := func() error {
		//invalidate the cache entity and drop all tracked dependencies
		if err := cr.cacheDep.Invalidate().WithCacheID(id).Exec(false); err != nil {
			return err
		}

		//as cache dependencies are optional we cannot rely that the previous
		//invalidation dropped the cache entity: delete the entity also explicitly
		q, err := db.NewQuery(cr.conn, &CacheEntryEntity{})
		if err != nil {
			return err
		}
		deleted, err := q.Delete().
			Where(map[string]interface{}{"ID": id}).
			Exec()
		cr.logger.Debug(fmt.Sprintf("Deleted %d cache entries which had no dependencies", deleted))
		return err
	}
	return cr.transactional(dbOps)
}
