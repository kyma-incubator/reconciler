package config

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type CacheEntryEntity struct {
	CacheID  string    `db:"notNull"`
	Cluster  string    `db:"notNull"`
	Buckets  string    `db:"notNull"`
	Cache    string    `db:"notNull"`
	Checksum string    `db:"notNull"`
	Created  time.Time `db:"readOnly"`
}

func (ce *CacheEntryEntity) String() string {
	return fmt.Sprintf("CacheID=%s,Cluster=%s,Buckets=%s,CreatedOn=%s",
		ce.CacheID, ce.Cluster, ce.Buckets, ce.Created)
}

func (ce *CacheEntryEntity) New() db.DatabaseEntity {
	return &CacheEntryEntity{}
}

func (ce *CacheEntryEntity) Synchronizer() *db.EntitySynchronizer {
	syncer := db.NewEntitySynchronizer(&ce)
	syncer.AddConverter("Created", convertTimestampToTime)
	return syncer
}

func (ce *CacheEntryEntity) Table() string {
	return tblCache
}

func (ce *CacheEntryEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherValue, ok := other.(*CacheEntryEntity)
	if ok {
		return ce.CacheID == otherValue.CacheID &&
			ce.Cluster == otherValue.Cluster
	}
	return false
}
