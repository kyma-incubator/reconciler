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

func (cr *CacheRepository) Add(cacheEntry *CacheEntryEntity) (*CacheEntryEntity, error) {
	q, err := db.NewQuery(cr.conn, cacheEntry)
	if err != nil {
		return nil, err
	}
	inCache, err := cr.Get(cacheEntry.Label, cacheEntry.Cluster)
	if err != nil && !IsNotFoundError(err) {
		return nil, err
	}
	if inCache != nil {
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
	return cacheEntry, q.Insert().Exec()
}

func (cr *CacheRepository) Invalidate(label, cluster string) error {
	q, err := db.NewQuery(cr.conn, &CacheEntryEntity{})
	if err != nil {
		return err
	}
	deleted, err := q.Delete().
		Where(map[string]interface{}{"Label": label, "Cluster": cluster}).
		Exec()
	if err == nil && deleted != 1 {
		cr.logger.Info(fmt.Sprintf("Invalidating cache entry with label '%s' and cluster '%s' returned "+
			"unexpected amount of deleted entries: got '%d' but expected 1",
			label, cluster, deleted))
	}
	return err
}

func (cr *CacheRepository) InvalidateByID(id int64) error {
	q, err := db.NewQuery(cr.conn, &CacheEntryEntity{})
	if err != nil {
		return err
	}
	deleted, err := q.Delete().
		Where(map[string]interface{}{"ID": id}).
		Exec()
	if err == nil && deleted != 1 {
		cr.logger.Info(fmt.Sprintf("Invalidating cache entry with id '%d' returned "+
			"unexpected amount of deleted entries: got '%d' but expected 1",
			id, deleted))
	}
	return err
}
