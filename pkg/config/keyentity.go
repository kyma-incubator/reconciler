package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/interpreter"
)

const (
	String  DataType = "string"
	Integer DataType = "integer"
	Boolean DataType = "boolean"
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
		typedValue, err = strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			return ke.fireParseError(value)
		}
	case Integer:
		typedValue, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return ke.fireParseError(value)
		}
	default:
		typedValue = value
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
			return &InvalidValueError{
				Key:       ke.Key,
				Value:     value,
				Validator: ke.Validator,
				Result:    result,
			}
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

func (ke *KeyEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&ke)
	marshaller.AddUnmarshaller("DataType", func(value interface{}) (interface{}, error) {
		return NewDataType(value.(string))
	})
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
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

type InvalidValueError struct {
	Validator string
	Result    interface{}
	Key       string
	Value     string
}

func (err InvalidValueError) Error() string {
	return fmt.Sprintf("Validation defined in key '%s' failed for value '%s':\n%s = %v", err.Key, err.Value, err.Validator, err.Result)
}

func IsInvalidValueError(err error) bool {
	return reflect.TypeOf(err) == reflect.TypeOf(&InvalidValueError{})
}
