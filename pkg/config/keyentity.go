package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/interpreter"
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

func (ke *KeyEntity) Validate(value string) error {
	var typedValue interface{}
	var err error

	//ensure data type
	switch ke.DataType {
	case Boolean:
		typedValue, err = strconv.ParseBool(value)
		if err != nil {
			return ke.fireParseError(value)
		}
	case Integer:
		typedValue, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return ke.fireParseError(value)
		}
	}

	//run validator logic for value
	if ke.Validator != "" {
		interp := interpreter.NewGolangInterpreter(ke.Validator).WithBindings(
			map[string]interface{}{"it": typedValue, "value": typedValue})
		result, err := interp.EvalBool()
		if err != nil {
			return err
		}
		if !result {
			return fmt.Errorf("Validation defined in key '%s' failed for value '%s'", ke.Key, value)
		}
	}

	return nil
}

func (ke *KeyEntity) fireParseError(value string) error {
	return fmt.Errorf("Key '%s' expects a value of type %s: provide value was '%s'", ke.Key, ke.DataType, value)
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
	syncer.AddConverter("Created", convertTimestampToTime)
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
