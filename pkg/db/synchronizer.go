package db

import (
	"fmt"
	"reflect"

	"github.com/fatih/structs"
)

type EntitySynchronizer struct {
	structs       *structs.Struct
	converterFcts map[string]func(rawValue interface{}) (interface{}, error)
}

func NewEntitySynchronizer(entity interface{}) *EntitySynchronizer {
	return &EntitySynchronizer{
		structs:       structs.New(entity),
		converterFcts: make(map[string]func(rawValue interface{}) (interface{}, error)),
	}
}

func (es *EntitySynchronizer) AddConverter(field string, convFct func(rawValue interface{}) (interface{}, error)) {
	es.converterFcts[field] = convFct
}

func (es *EntitySynchronizer) Sync(rawData map[string]interface{}) error {
	for _, field := range es.structs.Fields() {
		rawValue, ok := rawData[field.Name()]
		if !ok {
			return fmt.Errorf("No value in database found for field '%s'", field.Name())
		}

		//check if a value converter function was defined for this field
		if converterFct, ok := es.converterFcts[field.Name()]; ok {
			value, err := converterFct(rawValue)
			if err != nil {
				return err
			}
			if err := field.Set(value); err != nil {
				return err
			}
			continue
		}

		//set value without conversion
		if err := es.setFieldValue(field, rawValue); err != nil {
			return err
		}
	}
	return nil
}

func (es *EntitySynchronizer) setFieldValue(field *structs.Field, rawValue interface{}) error {
	var err error
	switch field.Kind() {
	case reflect.Int64:
		int64Value, ok := rawValue.(int64)
		if !ok {
			return es.fireCastError(reflect.Int64, field, rawValue)
		}
		err = field.Set(int64Value)
	case reflect.Bool:
		//some DBs handle booleans als integer values (0/1)
		boolValue, ok := rawValue.(bool)
		if !ok {
			int64Value, ok := rawValue.(int64)
			if !ok {
				return es.fireCastError(reflect.Bool, field, rawValue)
			}
			boolValue = (int64Value > 0)
		}
		err = field.Set(boolValue)
	case reflect.String:
		stringValue, ok := rawValue.(string)
		if !ok {
			return es.fireCastError(reflect.String, field, rawValue)
		}
		err = field.Set(stringValue)
	default:
		err = fmt.Errorf("Cannot synchronize field '%s' because type '%s' is not supported (value was '%s')",
			field.Name(), field.Kind(), rawValue)
	}
	return err
}

func (es *EntitySynchronizer) fireCastError(kind reflect.Kind, field *structs.Field, rawValue interface{}) error {
	return fmt.Errorf("Failed to convert value from DB to %s: got '%v' as value for entity field '%s'",
		kind.String(), rawValue, field.Name())
}
