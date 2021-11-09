package cache

import (
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

func (cr *Repository) All() ([]*model.CacheEntryEntity, error) {
	q, err := db.NewQuery(cr.Conn, &model.CacheEntryEntity{}, cr.Logger)
	if err != nil {
		return nil, err
	}
	entities, err := q.Select().
		GetMany()
	if err != nil {
		return nil, err
	}

	var result []*model.CacheEntryEntity
	for _, entity := range entities {
		result = append(result, entity.(*model.CacheEntryEntity))
	}
	return result, nil
}

func (cr *Repository) Get(label, runtimeID string) (*model.CacheEntryEntity, error) {
	q, err := db.NewQuery(cr.Conn, &model.CacheEntryEntity{}, cr.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"Label": label, "RuntimeID": runtimeID}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cr.NewNotFoundError(err, &model.CacheEntryEntity{}, whereCond)
	}
	return entity.(*model.CacheEntryEntity), nil
}

func (cr *Repository) GetByID(id int64) (*model.CacheEntryEntity, error) {
	q, err := db.NewQuery(cr.Conn, &model.CacheEntryEntity{}, cr.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{"ID": id}
	entity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, cr.NewNotFoundError(err, &model.CacheEntryEntity{}, whereCond)
	}
	return entity.(*model.CacheEntryEntity), nil
}

func (cr *Repository) Add(cacheEntry *model.CacheEntryEntity, cacheDeps []*model.ValueEntity) (*model.CacheEntryEntity, error) {
	//Get exiting cache entry
	inCache, err := cr.Get(cacheEntry.Label, cacheEntry.RuntimeID)
	if err != nil && !repository.IsNotFoundError(err) {
		return nil, err
	}
	if inCache != nil {
		//check if the existing cache entry is still valid otherwise invalidate it
		if inCache.Equal(cacheEntry) {
			cr.Logger.Debugf("No differences found for cache entry '%s' (cluster '%s'): not creating new database entity",
				cacheEntry.Label, cacheEntry.RuntimeID)
			return inCache, nil
		}
		cr.Logger.Debugf("Existing cache entry '%s' is outdated and will be invalidated", inCache)
		if err := cr.InvalidateByID(inCache.ID); err != nil {
			return cacheEntry, err
		}
	}

	//create new cache entry and track its dependencies
	dbOps := func() (interface{}, error) {
		q, err := db.NewQuery(cr.Conn, cacheEntry, cr.Logger)
		if err != nil {
			return cacheEntry, err
		}
		if err := q.Insert().Exec(); err != nil {
			return cacheEntry, err
		}
		if err := cr.CacheDep.Record(cacheEntry, cacheDeps).Exec(false); err != nil {
			return cacheEntry, err
		}
		return cacheEntry, err
	}

	var cacheEntryEntity *model.CacheEntryEntity
	result, err := cr.TransactionalResult(dbOps)
	if result != nil {
		cacheEntryEntity = result.(*model.CacheEntryEntity)
	}
	return cacheEntryEntity, err
}

func (cr *Repository) Invalidate(label, runtimeID string) error {
	dbOps := func() error {
		//invalidate the cache entity and drop all tracked dependencies
		if err := cr.CacheDep.Invalidate().WithLabel(label).WithRuntimeID(runtimeID).Exec(false); err != nil {
			return err
		}

		//as cache dependencies are optional we cannot rely that the previous
		//invalidation dropped the cache entity: delete the entity also explicitly
		q, err := db.NewQuery(cr.Conn, &model.CacheEntryEntity{}, cr.Logger)
		if err != nil {
			return err
		}
		deleted, err := q.Delete().
			Where(map[string]interface{}{"Label": label, "RuntimeID": runtimeID}).
			Exec()
		cr.Logger.Debugf("Deleted %d cache entries which had no dependencies", deleted)
		return err
	}
	return cr.Transactional(dbOps)
}

func (cr *Repository) InvalidateByID(id int64) error {
	dbOps := func() error {
		//invalidate the cache entity and drop all tracked dependencies
		if err := cr.CacheDep.Invalidate().WithCacheID(id).Exec(false); err != nil {
			return err
		}

		//as cache dependencies are optional we cannot rely that the previous
		//invalidation dropped the cache entity: delete the entity also explicitly
		q, err := db.NewQuery(cr.Conn, &model.CacheEntryEntity{}, cr.Logger)
		if err != nil {
			return err
		}
		deleted, err := q.Delete().
			Where(map[string]interface{}{"ID": id}).
			Exec()
		cr.Logger.Debugf("Deleted %d cache entries which had no dependencies", deleted)
		return err
	}
	return cr.Transactional(dbOps)
}
