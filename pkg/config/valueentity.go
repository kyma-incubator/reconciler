package config

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type ValueEntity struct {
	Key        string    `db:"notNull"`
	KeyVersion int64     `db:"notNull"`
	Version    int64     `db:"readOnly"`
	Bucket     string    `db:"notNull"`
	Value      string    `db:"notNull"`
	DataType   DataType  `db:"notNull"`
	Created    time.Time `db:"readOnly"`
	Username   string    `db:"notNull"`
}

func (ve *ValueEntity) String() string {
	return fmt.Sprintf("%s=%s: KeyVersion=%d,Bucket=%s,User=%s,CreatedOn=%s",
		ve.Key, ve.Value, ve.KeyVersion, ve.Bucket, ve.Username, ve.Created)
}

func (ve *ValueEntity) New() db.DatabaseEntity {
	return &ValueEntity{}
}

func (ve *ValueEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&ve)
	marshaller.AddUnmarshaller("DataType", convertStringToDataType)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	marshaller.AddMarshaller("Bucket", requireValidBucketName)
	return marshaller
}

func (ve *ValueEntity) Table() string {
	return tblValues
}

func (ve *ValueEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherValue, ok := other.(*ValueEntity)
	if ok {
		return ve.Bucket == otherValue.Bucket &&
			ve.Key == otherValue.Key &&
			ve.KeyVersion == otherValue.KeyVersion &&
			ve.Value == otherValue.Value
	}
	return false
}

func (ve *ValueEntity) Get() (interface{}, error) {
	return ve.DataType.Get(ve.Value)
}
