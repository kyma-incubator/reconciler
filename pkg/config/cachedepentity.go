package config

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type CacheDependencyEntity struct {
	Bucket  string    `db:"notNull"`
	Key     string    `db:"notNull"`
	Label   string    `db:"notNull"`
	Cluster string    `db:"notNull"`
	Created time.Time `db:"readOnly"`
}

func (cde *CacheDependencyEntity) String() string {
	return fmt.Sprintf("Bucket=%s,Key=%s,Label=%s,Cluster=%s,CreatedOn=%s",
		cde.Bucket, cde.Key, cde.Label, cde.Cluster, cde.Created)
}

func (cde *CacheDependencyEntity) New() db.DatabaseEntity {
	return &CacheDependencyEntity{}
}

func (cde *CacheDependencyEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&cde)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (cde *CacheDependencyEntity) Table() string {
	return tblCacheDeps
}

func (cde *CacheDependencyEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherDep, ok := other.(*CacheDependencyEntity)
	if ok {
		return cde.Bucket == otherDep.Bucket &&
			cde.Key == otherDep.Key &&
			cde.Label == otherDep.Label &&
			cde.Cluster == otherDep.Cluster
	}
	return false
}
