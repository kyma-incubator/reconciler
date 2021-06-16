package model

import (
	"fmt"
	"regexp"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const DefaultBucket = "default"

var bucketPattern = regexp.MustCompile(fmt.Sprintf(`^(%s|([a-z0-9]+(-[a-z0-9]+)+))$`, DefaultBucket))

type BucketEntity struct {
	Bucket   string    `db:"readOnly"`
	Created  time.Time `db:"readOnly"`
	Username string    `db:"readOnly"`
}

func (b *BucketEntity) String() string {
	return fmt.Sprintf("BucketEntity [Bucket=%s,User=%s]",
		b.Bucket, b.Username)
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

func ValidateBucketName(bucket string) error {
	if bucketPattern.MatchString(bucket) {
		return nil
	}
	return fmt.Errorf("Bucket name '%s' is invalid: bucket name has to match the pattern '%s'", bucket, bucketPattern)
}
