package config

import (
	"crypto/md5"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type CacheEntryEntity struct {
	CacheID  string    `db:"notNull"`
	Cluster  string    `db:"notNull"`
	Buckets  string    `db:"notNull"`
	Data     string    `db:"notNull"`
	checksum string    `db:"notNull"`
	Created  time.Time `db:"readOnly"`
}

func (ce *CacheEntryEntity) String() string {
	return fmt.Sprintf("CacheID=%s,Cluster=%s,Buckets=%s,CreatedOn=%s",
		ce.CacheID, ce.Cluster, ce.Buckets, ce.Created)
}

func (ce *CacheEntryEntity) New() db.DatabaseEntity {
	return &CacheEntryEntity{}
}

func (ce *CacheEntryEntity) Checksum() string {
	if ce.checksum == "" && ce.Data != "" {
		ce.checksum = ce.md5()
	}
	return ce.checksum
}

func (ce *CacheEntryEntity) md5() string {
	md5 := md5.Sum([]byte(ce.Data)) //nolint - MD5 is used for change detection, not for encryption
	return fmt.Sprintf("%x", md5)
}

func (ce *CacheEntryEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&ce)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	marshaller.AddMarshaller("checksum", func(value interface{}) (interface{}, error) {
		//ensure checksum is updated before entity got stored
		return ce.md5(), nil
	})
	return marshaller
}

func (ce *CacheEntryEntity) Table() string {
	return tblCache
}

func (ce *CacheEntryEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherEntry, ok := other.(*CacheEntryEntity)
	if ok {
		return ce.CacheID == otherEntry.CacheID &&
			ce.Cluster == otherEntry.Cluster &&
			ce.Checksum() == otherEntry.Checksum()
	}
	return false
}
