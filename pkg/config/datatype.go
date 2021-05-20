package config

import (
	"fmt"
	"strconv"
	"strings"
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

func (dt DataType) Get(value string) (interface{}, error) {
	var err error
	var typedValue interface{}
	switch dt {
	case Boolean:
		typedValue, err = strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			return typedValue, dt.fireParseError(value)
		}
	case Integer:
		typedValue, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return typedValue, dt.fireParseError(value)
		}
	default:
		typedValue = value
	}
	return typedValue, nil
}

func (dt DataType) fireParseError(value string) error {
	return fmt.Errorf("Value '%s' is not compatible with DataType '%s'", value, dt)
}
