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

type IncompleteEntityError struct {
	errorMsg string
}

func (e *IncompleteEntityError) Error() string {
	return e.errorMsg
}

func newIncompleteEntityError(err string, args ...interface{}) *IncompleteEntityError {
	return &IncompleteEntityError{
		errorMsg: fmt.Sprintf(err, args...),
	}
}

func IsIncompleteEntityError(err error) bool {
	_, ok := err.(*IncompleteEntityError)
	return ok
}

type column struct {
	name     string
	readOnly bool
	notNull  bool
	field    *structs.Field
	value    interface{}
}

type ColumnHandler struct {
	columns     []*column
	columnNames map[string]string
}

func NewColumnHandler(entity interface{}) (*ColumnHandler, error) {
	kind := reflect.ValueOf(entity).Kind()
	if kind == reflect.Ptr {
		kind = reflect.ValueOf(entity).Elem().Kind()
	}
	if kind != reflect.Struct {
		return nil, fmt.Errorf("ColumnHandler accepts only structs but provided object was '%s'", kind)
	}
	fields := structs.Fields(entity)
	stc := &ColumnHandler{
		columnNames: make(map[string]string, len(fields)),
	}
	for _, field := range fields {
		col := &column{
			name:     strcase.ToSnake(field.Name()),
			readOnly: hasTag(field, dbTagReadOnly),
			notNull:  hasTag(field, dbTagNoNull),
			field:    field,
		}
		stc.columns = append(stc.columns, col)
		stc.columnNames[field.Name()] = col.name
	}
	return stc, nil
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

func (tc *ColumnHandler) Validate() error {
	invalidFields := []string{}
	for _, col := range tc.columns {
		if col.notNull {
			switch col.field.Kind() {
			case reflect.String:
				if fmt.Sprintf("%s", col.field.Value()) == "" {
					invalidFields = append(invalidFields, col.field.Name())
				}
			case reflect.Int64:
				if col.field.Value().(int64) == 0 {
					invalidFields = append(invalidFields, col.field.Name())
				}
			case reflect.Bool:
				//nothing to check
			default:
				return fmt.Errorf("Field '%s' has type '%s' - this type is not supported yet",
					col.field.Name(), col.field.Kind())
			}
		}
	}
	if len(invalidFields) > 0 {
		return newIncompleteEntityError("The fields '%s' are tagged with '%s' and cannot be undefined",
			strings.Join(invalidFields, "', '"), dbTagNoNull)
	}
	return nil
}

func (tc *ColumnHandler) ColumnName(field string) (string, error) {
	if colName, ok := tc.columnNames[field]; ok {
		return colName, nil
	}
	return "", fmt.Errorf("Entity has no field '%s' cannot resolve column name", field)
}

//ColumnNamesCsv returns the CSV string of the column names
func (tc *ColumnHandler) ColumnNamesCsv(onlyWriteable bool) string {
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

func (tc *ColumnHandler) ColumnValues(onlyWriteable bool) []interface{} {
	result := []interface{}{}
	for _, col := range tc.columns {
		if onlyWriteable && col.readOnly {
			continue
		}
		result = append(result, col.field.Value())
	}
	return result
}

func (tc *ColumnHandler) columnValuesCsvRenderer(onlyWriteable, placeholder bool) string {
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
			buffer.WriteString(tc.serializeValue(col.field.Value()))
		}

	}
	return buffer.String()
}

func (tc *ColumnHandler) ColumnValuesCsv(onlyWriteable bool) string {
	return tc.columnValuesCsvRenderer(onlyWriteable, false)
}

func (tc *ColumnHandler) ColumnValuesPlaceholderCsv(onlyWriteable bool) string {
	return tc.columnValuesCsvRenderer(onlyWriteable, true)
}

func (tc *ColumnHandler) columnEntriesCsvRenderer(onlyWriteable, placeholder bool) string {
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
			buffer.WriteString(fmt.Sprintf("%s=%s", col.name, tc.serializeValue(col.field.Value())))
		}
	}
	return buffer.String()
}

func (tc *ColumnHandler) ColumnEntriesCsv(onlyWriteable bool) string {
	return tc.columnEntriesCsvRenderer(onlyWriteable, false)
}

func (tc *ColumnHandler) ColumnEntriesPlaceholderCsv(onlyWriteable bool) string {
	return tc.columnEntriesCsvRenderer(onlyWriteable, true)
}

func (tc *ColumnHandler) serializeValue(value interface{}) string {
	switch reflect.ValueOf(value).Kind() {
	case reflect.Bool:
		return fmt.Sprintf("%t", value)
	case reflect.Int64:
		return fmt.Sprintf("%d", value)
	default:
		return fmt.Sprintf("'%s'", value)
	}
}

func (tc *ColumnHandler) Synchronize(row DataRow, entity DatabaseEntity) error {
	colVals := []interface{}{}
	for _, col := range tc.columns {
		colVals = append(colVals, &col.value)
	}
	if err := row.Scan(colVals...); err != nil {
		return err
	}
	entityData := make(map[string]interface{}, len(tc.columns))
	for _, col := range tc.columns {
		entityData[col.field.Name()] = col.value
	}
	return entity.Synchronizer().Sync(entityData)
}
