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
	dbTagNotNull  string = "notNull"
)

type InvalidEntityError struct {
	errorMsg string
}

func (e *InvalidEntityError) Error() string {
	return e.errorMsg
}

func newInvalidEntityError(err string, args ...interface{}) *InvalidEntityError {
	return &InvalidEntityError{
		errorMsg: fmt.Sprintf(err, args...),
	}
}

func IsInvalidEntityError(err error) bool {
	_, ok := err.(*InvalidEntityError)
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
	entity      DatabaseEntity
	columns     []*column
	columnNames map[string]string //cache for column names (to increase lookup speed)
}

func NewColumnHandler(entity DatabaseEntity) (*ColumnHandler, error) {
	//create column handler instance
	fields := structs.Fields(entity)
	colHdlr := &ColumnHandler{
		entity:      entity,
		columnNames: make(map[string]string, len(fields)),
	}

	//get marshalled values of entity fields
	marshalledValues, err := entity.Marshaller().Marshal()
	if err != nil {
		return colHdlr, newInvalidEntityError(fmt.Sprintf("Failed to marshal values of entity '%s': %s", entity, err.Error()))
	}

	//add columns to column handler instance
	for _, field := range fields {
		col := &column{
			name:     strcase.ToSnake(field.Name()),
			readOnly: hasTag(field, dbTagReadOnly),
			notNull:  hasTag(field, dbTagNotNull),
			field:    field,
			value:    marshalledValues[field.Name()],
		}
		colHdlr.columns = append(colHdlr.columns, col)
		colHdlr.columnNames[field.Name()] = col.name
	}

	return colHdlr, nil
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

func (ch *ColumnHandler) Validate() error {
	invalidFields := []string{}
	for _, col := range ch.columns {
		if col.notNull {
			switch col.field.Kind() {
			case reflect.String:
				if fmt.Sprintf("%s", col.value) == "" {
					invalidFields = append(invalidFields, col.field.Name())
				}
			case reflect.Int:
				if col.value.(int) == 0 {
					invalidFields = append(invalidFields, col.field.Name())
				}
			case reflect.Int64:
				if col.value.(int64) == 0 {
					invalidFields = append(invalidFields, col.field.Name())
				}
			case reflect.Float64:
				if col.value.(float64) == 0 {
					invalidFields = append(invalidFields, col.field.Name())
				}
			case reflect.Bool:
				//nothing to check
			default:
				return fmt.Errorf("Field '%s' of entity '%s' has type '%s' - this type is not supported yet",
					col.field.Name(), ch.entity, col.field.Kind())
			}
		}
	}
	if len(invalidFields) > 0 {
		return newInvalidEntityError("The fields '%s' of entity '%s' are tagged with '%s' and cannot be undefined",
			strings.Join(invalidFields, "', '"), ch.entity, dbTagNotNull)
	}
	return nil
}

func (ch *ColumnHandler) ColumnName(field string) (string, error) {
	if colName, ok := ch.columnNames[field]; ok {
		return colName, nil
	}
	return "", fmt.Errorf("Entity '%s' has no field '%s': cannot resolve column name", ch.entity, field)
}

//ColumnNamesCsv returns the CSV string of the column names
func (ch *ColumnHandler) ColumnNamesCsv(onlyWriteable bool) string {
	var buffer bytes.Buffer
	for _, col := range ch.columns {
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

func (ch *ColumnHandler) ColumnValues(onlyWriteable bool) []interface{} {
	result := []interface{}{}
	for _, col := range ch.columns {
		if onlyWriteable && col.readOnly {
			continue
		}
		result = append(result, col.value)
	}
	return result
}

func (ch *ColumnHandler) columnValuesCsvRenderer(onlyWriteable, placeholder bool) string {
	var buffer bytes.Buffer
	var placeholderIdx int
	for _, col := range ch.columns {
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
			buffer.WriteString(ch.serializeValue(col.value))
		}

	}
	return buffer.String()
}

func (ch *ColumnHandler) ColumnValuesCsv(onlyWriteable bool) string {
	return ch.columnValuesCsvRenderer(onlyWriteable, false)
}

func (ch *ColumnHandler) ColumnValuesPlaceholderCsv(onlyWriteable bool) string {
	return ch.columnValuesCsvRenderer(onlyWriteable, true)
}

func (ch *ColumnHandler) columnEntriesCsvRenderer(onlyWriteable, placeholder bool) (string, int) {
	var buffer bytes.Buffer
	var placeholderIdx int
	for _, col := range ch.columns {
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
			buffer.WriteString(fmt.Sprintf("%s=%s", col.name, ch.serializeValue(col.value)))
		}
	}
	return buffer.String(), placeholderIdx
}

func (ch *ColumnHandler) ColumnEntriesCsv(onlyWriteable bool) (string, int) {
	return ch.columnEntriesCsvRenderer(onlyWriteable, false)
}

func (ch *ColumnHandler) ColumnEntriesPlaceholderCsv(onlyWriteable bool) (string, int) {
	return ch.columnEntriesCsvRenderer(onlyWriteable, true)
}

func (ch *ColumnHandler) serializeValue(value interface{}) string {
	switch reflect.ValueOf(value).Kind() {
	case reflect.Bool:
		return fmt.Sprintf("%t", value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", value)
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%f", value)
	default:
		return fmt.Sprintf("'%v'", value)
	}
}

func (ch *ColumnHandler) Unmarshal(row DataRow, entity DatabaseEntity) error {
	colVals := []interface{}{}
	for _, col := range ch.columns {
		colVals = append(colVals, &col.value)
	}
	if err := row.Scan(colVals...); err != nil {
		return err
	}
	entityData := make(map[string]interface{}, len(ch.columns))
	for _, col := range ch.columns {
		entityData[col.field.Name()] = col.value
	}
	return entity.Marshaller().Unmarshal(entityData)
}
