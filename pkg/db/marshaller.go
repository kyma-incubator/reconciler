package db

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
)

type EntityMarshaller struct {
	structs       *structs.Struct
	marshalFcts   map[string]func(value interface{}) (interface{}, error)
	unmarshalFcts map[string]func(value interface{}) (interface{}, error)
}

func NewEntityMarshaller(entity interface{}) *EntityMarshaller {
	return &EntityMarshaller{
		structs:       structs.New(entity),
		marshalFcts:   make(map[string]func(value interface{}) (interface{}, error)),
		unmarshalFcts: make(map[string]func(value interface{}) (interface{}, error)),
	}
}

func (es *EntityMarshaller) AddMarshaller(field string, fct func(value interface{}) (interface{}, error)) {
	es.ensureFieldExist(field)
	es.marshalFcts[field] = fct
}

func (es *EntityMarshaller) AddUnmarshaller(field string, fct func(value interface{}) (interface{}, error)) {
	es.ensureFieldExist(field)
	es.unmarshalFcts[field] = fct
}

func (es *EntityMarshaller) ensureFieldExist(field string) {
	if _, ok := es.structs.FieldOk(field); !ok {
		panic(fmt.Sprintf("Failure in Marshaller: the entity '%s' has not field '%s'", es.structs.Name(), field))
	}
}

func (es *EntityMarshaller) Marshal() (map[string]interface{}, error) {
	var err error
	result := make(map[string]interface{}, len(es.structs.Fields()))
	for _, field := range es.structs.Fields() {
		fieldName := field.Name()
		marshalledValue := field.Value()
		if fct, ok := es.marshalFcts[fieldName]; ok {
			marshalledValue, err = fct(marshalledValue)
			if err != nil {
				break
			}
		}
		result[field.Name()] = marshalledValue
	}
	return result, err
}

func (es *EntityMarshaller) Unmarshal(rawData map[string]interface{}) error {
	for _, field := range es.structs.Fields() {
		value, ok := rawData[field.Name()]
		if !ok {
			return fmt.Errorf("No value in database found for field '%s'", field.Name())
		}

		//check if a value converter function was defined for this field
		if fct, ok := es.unmarshalFcts[field.Name()]; ok {
			value, err := fct(value)
			if err != nil {
				return err
			}
			if err := field.Set(value); err != nil {
				return err
			}
			continue
		}

		//set value without conversion
		if err := es.setFieldValue(field, value); err != nil {
			return err
		}
	}
	return nil
}

func (es *EntityMarshaller) setFieldValue(field *structs.Field, value interface{}) error {
	var err error
	switch field.Kind() {
	case reflect.Int:
		intValue, ok := value.(int)
		if !ok {
			return es.fireCastError(reflect.Int, field, value)
		}
		err = field.Set(intValue)
	case reflect.Int64:
		int64Value, ok := value.(int64)
		if !ok {
			return es.fireCastError(reflect.Int64, field, value)
		}
		err = field.Set(int64Value)
	case reflect.Float64:
		float64Value, ok := value.(float64)
		if !ok {
			return es.fireCastError(reflect.Float64, field, value)
		}
		err = field.Set(float64Value)
	case reflect.Bool:
		var boolValue bool
		switch strings.ToLower(fmt.Sprintf("%v", value)) {
		case "true":
			boolValue = true
		case "1":
			boolValue = true
		case "false":
			boolValue = false
		case "0":
			boolValue = false
		default:
			return es.fireCastError(reflect.Bool, field, value)
		}
		err = field.Set(boolValue)
	case reflect.String:
		stringValue, ok := value.(string)
		if !ok {
			return es.fireCastError(reflect.String, field, value)
		}
		err = field.Set(stringValue)
	default:
		err = fmt.Errorf("Cannot synchronize field '%s' because type '%s' is not supported (value was '%v')",
			field.Name(), field.Kind(), value)
	}
	return err
}

func (es *EntityMarshaller) fireCastError(kind reflect.Kind, field *structs.Field, value interface{}) error {
	return fmt.Errorf("Failed to convert value from DB to %s: got '%v' as value for entity field '%s'",
		kind.String(), value, field.Name())
}
