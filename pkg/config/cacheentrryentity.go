package config

import (
	"crypto/md5"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type CacheEntryEntity struct {
	ID       int64     `db:"readOnly"`
	Label    string    `db:"notNull"`
	Cluster  string    `db:"notNull"`
	Data     string    `db:"notNull"`
	Checksum string    `db:"notNull"`
	Created  time.Time `db:"readOnly"`
}

func (ce *CacheEntryEntity) String() string {
	return fmt.Sprintf("Label=%s,Cluster=%s,Checksum=%s,CreatedOn=%s",
		ce.Label, ce.Cluster, ce.checksum(), ce.Created)
}

func (ce *CacheEntryEntity) New() db.DatabaseEntity {
	return &CacheEntryEntity{}
}

func (ce *CacheEntryEntity) checksum() string {
	if ce.Checksum == "" && ce.Data != "" {
		ce.Checksum = ce.md5()
	}
	return ce.Checksum
}

func (ce *CacheEntryEntity) md5() string {
	md5 := md5.Sum([]byte(ce.Data)) //nolint - MD5 is used for change detection, not for encryption
	return fmt.Sprintf("%x", md5)
}

func (ce *CacheEntryEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&ce)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	marshaller.AddMarshaller("Checksum", func(value interface{}) (interface{}, error) {
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
		return ce.Label == otherEntry.Label &&
			ce.Cluster == otherEntry.Cluster &&
			ce.checksum() == otherEntry.checksum()
	}
	return false
}
