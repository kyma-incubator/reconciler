package db

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/iancoleman/strcase"
)

type StructTableConverter struct {
	columns []string
	values  []interface{}
}

func NewStructTableConverter(obj interface{}) (*StructTableConverter, error) {
	kind := reflect.ValueOf(obj).Kind()
	if kind != reflect.Struct {
		return nil, fmt.Errorf("StructTableConverter accepts only structs but provided object was '%s'", kind)
	}
	colNames, colValues := columnNameValues(obj)
	return &StructTableConverter{
		columns: colNames,
		values:  colValues,
	}, nil
}

//Convert the fields of a struct to their corresponding table column names inclusive the field value
func structData(obj interface{}) map[string]interface{} {
	fields := structs.Fields(obj)
	result := make(map[string]interface{}, len(fields))
	for _, field := range fields {
		result[strcase.ToSnake(field.Name())] = field.Value()
	}
	return result
}

func columnNameValues(obj interface{}) ([]string, []interface{}) {
	structData := structData(obj)
	columnNames := []string{}
	columnValues := []interface{}{}
	for k, v := range structData {
		columnNames = append(columnNames, k)
		columnValues = append(columnValues, v)
	}
	return columnNames, columnValues
}

//ColumnNamesCsv returns the CSV string of the column names
func (tc *StructTableConverter) ColumnNamesCsv() string {
	return strings.Join(tc.columns, ", ")
}

//ColumnValuesCsv returns the CSV string of the column values
func (tc *StructTableConverter) ColumnValuesCsv() string {
	var buffer bytes.Buffer
	for _, value := range tc.values {
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(tc.serializeValue(value))
	}
	return buffer.String()
}

//ColumnEntryCsv returns the CSV strings of the column names-values pairs (e.g. col1=val1,col2=val2,...)
func (tc *StructTableConverter) ColumnEntriesCsv() string {
	var buffer bytes.Buffer
	for idx, colName := range tc.columns {
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(fmt.Sprintf("%s=%s", colName, tc.serializeValue(tc.values[idx])))
	}
	return buffer.String()
}

func (tc *StructTableConverter) serializeValue(value interface{}) string {
	switch reflect.ValueOf(value).Kind() {
	case reflect.Bool:
		return fmt.Sprintf("%t", value)
	case reflect.Int64:
		return fmt.Sprintf("%d", value)
	default:
		return fmt.Sprintf("'%s'", value)
	}
}
