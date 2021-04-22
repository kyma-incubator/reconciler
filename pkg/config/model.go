package config

import (
	"fmt"
	"time"

	"github.com/fatih/structs"
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
	//register custom value converter
	valConvs := make(map[string]func(interface{}) (interface{}, error))

	//convert for DataType field
	valConvs["DataType"] = func(value interface{}) (interface{}, error) {
		var dataType DataType
		switch value.(string) {
		case "integer":
			dataType = Integer
		case "string":
			dataType = String
		case "boolean":
			dataType = Boolean
		default:
			return nil, fmt.Errorf("Value '%s' is not a valid DataType - data inconsistency detected!", value)
		}
		return dataType, nil
	}

	//convert for Created field
	valConvs["Created"] = func(value interface{}) (interface{}, error) {
		return value, nil
	}

	return &db.EntitySynchronizer{
		Struct:         structs.New(&ke),
		ValueConverter: valConvs,
	}
}

func (ke *KeyEntity) Table() string {
	return tblKeys
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

func (ke *ValueEntity) New() db.DatabaseEntity {
	return &ValueEntity{}
}

func (ke *ValueEntity) Synchronizer() *db.EntitySynchronizer {
	//register custom value converter
	valConvs := make(map[string]func(interface{}) (interface{}, error))

	//convert for Created field
	valConvs["Created"] = func(value interface{}) (interface{}, error) {
		return value, nil
	}

	return &db.EntitySynchronizer{
		Struct:         structs.New(&ke),
		ValueConverter: valConvs,
	}
}

func (ke *ValueEntity) Table() string {
	return tlbValues
}
