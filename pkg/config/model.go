package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const (
	String    DataType = "string"
	Integer   DataType = "integer"
	Boolean   DataType = "boolean"
	tblKeys   string   = "config_keys"
	tlbValues string   = "config_values"
)

type DataType string

func NewDataType(dataType string) (DataType, error) {
	switch strings.ToLower(dataType) {
	case "string":
		return String, nil
	case "integer":
		return Integer, nil
	case "boolean":
		return Boolean, nil
	default:
		return "", fmt.Errorf("DataType '%s' is not supported", dataType)
	}
}

type KeyEntity struct {
	Key       string   `db:"notNull"`
	Version   int64    `db:"readOnly"`
	DataType  DataType `db:"notNull"`
	Encrypted bool
	Created   time.Time `db:"readOnly"`
	Username  string    `db:"notNull"`
	Validator string
	Trigger   string
}

func (ke *KeyEntity) String() string {
	return fmt.Sprintf("%s (v%d): Type=%s,Encrypted=%t,User=%s,CreatedOn=%s",
		ke.Key, ke.Version, ke.DataType, ke.Encrypted, ke.Username, ke.Created)
}

func (ke *KeyEntity) New() db.DatabaseEntity {
	return &KeyEntity{}
}

func (ke *KeyEntity) Synchronizer() *db.EntitySynchronizer {
	syncer := db.NewEntitySynchronizer(&ke)
	syncer.AddConverter("DataType", func(value interface{}) (interface{}, error) {
		return NewDataType(value.(string))
	})
	syncer.AddConverter("Created", createdToTime)
	return syncer
}

func (ke *KeyEntity) Table() string {
	return tblKeys
}

func (ke *KeyEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherKey, ok := other.(*KeyEntity)
	if ok {
		return ke.Key == otherKey.Key &&
			ke.DataType == otherKey.DataType &&
			ke.Encrypted == otherKey.Encrypted &&
			ke.Validator == otherKey.Validator &&
			ke.Trigger == otherKey.Trigger
	}
	return false
}

type ValueEntity struct {
	Key        string    `db:"notNull"`
	KeyVersion int64     `db:"notNull"`
	Version    int64     `db:"readOnly"`
	Bucket     string    `db:"notNull"`
	Value      string    `db:"notNull"`
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

func (ve *ValueEntity) Synchronizer() *db.EntitySynchronizer {
	syncer := db.NewEntitySynchronizer(&ve)
	syncer.AddConverter("Created", createdToTime)
	return syncer
}

func (ve *ValueEntity) Table() string {
	return tlbValues
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

func (b *BucketEntity) Synchronizer() *db.EntitySynchronizer {
	syncer := db.NewEntitySynchronizer(&b)
	syncer.AddConverter("Created", createdToTime)
	return syncer
}

func (b *BucketEntity) Table() string {
	return tlbValues
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

func createdToTime(value interface{}) (interface{}, error) {
	if reflect.TypeOf(value).Kind() == reflect.String {
		layout := "2006-02-01 15:04:05"
		return time.Parse(layout, value.(string))
	}
	if time, ok := value.(time.Time); ok {
		return time, nil
	}
	return nil, fmt.Errorf("Failed to convert value '%s' (kind: %s) for field 'Created' to Time struct",
		value, reflect.TypeOf(value).Kind())
}
