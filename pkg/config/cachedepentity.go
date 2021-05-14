package config

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type CacheDependencyEntity struct {
	Bucket  string    `db:"notNull"`
	Key     string    `db:"notNull"`
	CacheID string    `db:"notNull"`
	Cluster string    `db:"notNull"`
	Created time.Time `db:"readOnly"`
}

func (cde *CacheDependencyEntity) String() string {
	return fmt.Sprintf("Bucket=%s,Key=%s,CacheID=%s,Cluster=%s,CreatedOn=%s",
		cde.Bucket, cde.Key, cde.CacheID, cde.Cluster, cde.Created)
}

func (cde *CacheDependencyEntity) New() db.DatabaseEntity {
	return &CacheDependencyEntity{}
}

func (cde *CacheDependencyEntity) Synchronizer() *db.EntitySynchronizer {
	syncer := db.NewEntitySynchronizer(&cde)
	syncer.AddConverter("Created", convertTimestampToTime)
	return syncer
}

func (cde *CacheDependencyEntity) Table() string {
	return tblCacheDeps
}

func (cde *CacheDependencyEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherValue, ok := other.(*CacheDependencyEntity)
	if ok {
		return cde.Bucket == otherValue.Bucket &&
			cde.Key == otherValue.Key &&
			cde.CacheID == otherValue.CacheID &&
			cde.Cluster == otherValue.Cluster
	}
	return false
}
