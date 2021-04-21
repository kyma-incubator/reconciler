package db

import (
	"fmt"
	"reflect"

	"github.com/fatih/structs"
)

type StructSynchronizer struct {
	Struct         *structs.Struct
	ValueConverter map[string]func(interface{}) (interface{}, error)
}

func (sd *StructSynchronizer) Sync(data map[string]interface{}) error {
	for _, field := range sd.Struct.Fields() {
		rawValue, ok := data[field.Name()]
		if !ok {
			return fmt.Errorf("No value in database found for field '%s'", field.Name())
		}

		//check if a value converter function was defined for this field
		if valueConverterFct, ok := sd.ValueConverter[field.Name()]; ok {
			value, err := valueConverterFct(rawValue)
			if err != nil {
				return err
			}
			field.Set(value)
			continue
		}

		//use default value conversion
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
		if err != nil {
			return err
		}
	}

	return nil
}
