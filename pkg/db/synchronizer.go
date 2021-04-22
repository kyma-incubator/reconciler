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
		err = field.Set(rawValue.(int64))
	case reflect.Bool:
		err = field.Set(rawValue.(bool))
	case reflect.String:
		err = field.Set(rawValue.(string))
	default:
		err = fmt.Errorf("Cannot synchronize field '%s' because type '%s' is not supported (value was '%s')",
			field.Name(), field.Kind(), rawValue)
	}
	return err
}
