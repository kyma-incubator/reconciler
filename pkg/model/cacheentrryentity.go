package model

import (
	//nolint - ignore blacklisted import as we use md5 lib just for change detection and not for encryption
	"crypto/md5"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblCache string = "config_cache"

type CacheEntryEntity struct {
	ID        int64     `db:"readOnly"`
	Label     string    `db:"notNull"`
	RuntimeID string    `db:"notNull"`
	Data      string    `db:"notNull"`
	Checksum  string    `db:"notNull"`
	Created   time.Time `db:"readOnly"`
}

func (ce *CacheEntryEntity) String() string {
	return fmt.Sprintf("CacheEntryEntity [ID=%d,Label=%s,RuntimeID=%s,Checksum=%s]",
		ce.ID, ce.Label, ce.RuntimeID, ce.NewChecksum())
}

func (ce *CacheEntryEntity) New() db.DatabaseEntity {
	return &CacheEntryEntity{}
}

func (ce *CacheEntryEntity) NewChecksum() string {
	if ce.Checksum == "" && ce.Data != "" {
		ce.Checksum = ce.md5()
	}
	return ce.Checksum
}

func (ce *CacheEntryEntity) md5() string {
	chksum := md5.Sum([]byte(ce.Data)) //nolint - MD5 is used for change detection, not for encryption
	return fmt.Sprintf("%x", chksum)
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
			ce.RuntimeID == otherEntry.RuntimeID &&
			ce.NewChecksum() == otherEntry.NewChecksum()
	}
	return false
}
