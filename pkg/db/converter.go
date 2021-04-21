package db

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/iancoleman/strcase"
)

const (
	dbTag         string = "db"
	dbTagReadOnly string = "readOnly"
	dbTagNoNull   string = "notNull"
)

type column struct {
	name     string
	readOnly bool
	notNull  bool
	value    interface{}
}

type StructTableConverter struct {
	columns []*column
}

func NewStructTableConverter(obj interface{}) (*StructTableConverter, error) {
	kind := reflect.ValueOf(obj).Kind()
	if kind != reflect.Struct {
		return nil, fmt.Errorf("StructTableConverter accepts only structs but provided object was '%s'", kind)
	}
	cols, err := structToColumns(obj)
	if err != nil {
		return nil, err
	}
	return &StructTableConverter{
		columns: cols,
	}, nil
}

func structToColumns(obj interface{}) ([]*column, error) {
	fields := structs.Fields(obj)
	result := []*column{}
	for _, field := range fields {
		col := &column{
			name:     strcase.ToSnake(field.Name()),
			readOnly: hasTag(field, dbTagReadOnly),
			notNull:  hasTag(field, dbTagNoNull),
			value:    field.Value(),
		}
		if col.notNull {
			switch field.Kind() {
			case reflect.String:
				if field.Value().(string) == "" {
					return nil, fmt.Errorf("Field '%s' is tagged with 'notNull' and cannot be empty", field.Name())
				}
			case reflect.Int64:
				if field.Value().(int64) == 0 {
					return nil, fmt.Errorf("Field '%s' is tagged with 'notNull' and cannot be 0", field.Name())
				}
			case reflect.Bool:
				//nothing to check
			default:
				return nil, fmt.Errorf("Field '%s' has type '%s' - this type is not supported yet", field.Name(), field.Kind())
			}
		}
		result = append(result, col)
	}
	return result, nil
}

func hasTag(field *structs.Field, tag string) bool {
	tags := strings.Split(field.Tag(dbTag), ",")
	for _, t := range tags {
		if tag == strings.TrimSpace(t) {
			return true
		}
	}
	return false
}

//ColumnNamesCsv returns the CSV string of the column names
func (tc *StructTableConverter) ColumnNamesCsv(onlyWriteable bool) string {
	var buffer bytes.Buffer
	for _, col := range tc.columns {
		if onlyWriteable && col.readOnly {
			continue
		}
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(col.name)
	}
	return buffer.String()
}

func (tc *StructTableConverter) columnValuesCsvRenderer(onlyWriteable, placeholder bool) string {
	var buffer bytes.Buffer
	var placeholderIdx int
	for _, col := range tc.columns {
		if onlyWriteable && col.readOnly {
			continue
		}
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		if placeholder {
			placeholderIdx++
			buffer.WriteString(fmt.Sprintf("$%d", placeholderIdx))
		} else {
			buffer.WriteString(tc.serializeValue(col.value))
		}

	}
	return buffer.String()
}

func (tc *StructTableConverter) ColumnValues(onlyWriteable bool) []interface{} {
	result := []interface{}{}
	for _, col := range tc.columns {
		if onlyWriteable && col.readOnly {
			continue
		}
		result = append(result, col.value)
	}
	return result
}

func (tc *StructTableConverter) ColumnValuesCsv(onlyWriteable bool) string {
	return tc.columnValuesCsvRenderer(onlyWriteable, false)
}

func (tc *StructTableConverter) ColumnValuesPlaceholderCsv(onlyWriteable bool) string {
	return tc.columnValuesCsvRenderer(onlyWriteable, true)
}

func (tc *StructTableConverter) columnEntriesCsvRenderer(onlyWriteable, placeholder bool) string {
	var buffer bytes.Buffer
	var placeholderIdx int
	for _, col := range tc.columns {
		if onlyWriteable && col.readOnly {
			continue
		}
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		if placeholder {
			placeholderIdx++
			buffer.WriteString(fmt.Sprintf("%s=$%d", col.name, placeholderIdx))
		} else {
			buffer.WriteString(fmt.Sprintf("%s=%s", col.name, tc.serializeValue(col.value)))
		}
	}
	return buffer.String()
}

func (tc *StructTableConverter) ColumnEntriesCsv(onlyWriteable bool) string {
	return tc.columnEntriesCsvRenderer(onlyWriteable, false)
}

func (tc *StructTableConverter) ColumnEntriesPlaceholderCsv(onlyWriteable bool) string {
	return tc.columnEntriesCsvRenderer(onlyWriteable, true)
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
