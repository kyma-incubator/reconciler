package config

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type BucketEntity struct {
	Bucket   string    `db:"readOnly"`
	Created  time.Time `db:"readOnly"`
	Username string    `db:"readOnly"`
}

func (b *BucketEntity) String() string {
	return fmt.Sprintf("Bucket=%s,User=%s,CreatedOn=%s",
		b.Bucket, b.Username, b.Created)
}

func (b *BucketEntity) New() db.DatabaseEntity {
	return &BucketEntity{}
}

func (b *BucketEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&b)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (b *BucketEntity) Table() string {
	return tblValues
}

func (b *BucketEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherBucket, ok := other.(*BucketEntity)
	if ok {
		return b.Bucket == otherBucket.Bucket
	}
	return false
}
