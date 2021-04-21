package config

import "fmt"

const (
	String  DataType = "string"
	Integer DataType = "integer"
	Boolean DataType = "boolean"
)

type DataType string

type KeyEntity struct {
	Key       string   `db:"notNull"`
	Version   int64    `db:"readOnly"`
	DataType  DataType `db:"notNull"`
	Encrypted bool
	Created   int64  `db:"readOnly"`
	User      string `db:"notNull"`
	Validator string
	Trigger   string
}

func (ke *KeyEntity) String() string {
	return fmt.Sprintf("%s (v%d): Type=%s,Encrypted=%t,User=%s,CreatedOn=%d",
		ke.Key, ke.Version, ke.DataType, ke.Encrypted, ke.User, ke.Created)
}

type ValueEntity struct {
	Key        string `db:"notNull"`
	KeyVersion int64  `db:"notNull"`
	Version    int64  `db:"readOnly"`
	Bucket     string `db:"notNull"`
	Value      string `db:"notNull"`
	Created    int64  `db:"readOnly"`
	User       string `db:"notNull"`
}

func (ve *ValueEntity) String() string {
	return fmt.Sprintf("%s=%s: KeyVersion=%d,Bucket=%s,User=%s,CreatedOn=%d",
		ve.Key, ve.Value, ve.KeyVersion, ve.Bucket, ve.User, ve.Created)
}
