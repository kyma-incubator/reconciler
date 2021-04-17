package config

const (
	String  DataType = "string"
	Integer DataType = "integer"
	Boolean DataType = "boolean"
)

type DataType string

type KeyEntity struct {
	Key       string
	Version   int
	DataType  DataType
	Encrypted bool
	Created   int64
	User      string
	Validator string
	Trigger   string
}

type ValueEntity struct {
	Key        string
	KeyVersion int
	Version    int
	Bucket     string
	Value      string
	Created    int64
	User       string
}
