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
	dbTagEncrypt  string = "encrypt"
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
	encrypt  bool
	field    *structs.Field
	value    interface{}
}

type ColumnHandler struct {
	entity      DatabaseEntity
	encryptor   *Encryptor
	columns     []*column
	columnNames map[string]string //cache for column names (to increase lookup speed)
}

func NewColumnHandler(entity DatabaseEntity, conn Connection) (*ColumnHandler, error) {
	//new col handler instance
	fields := structs.Fields(entity)
	colHdlr := &ColumnHandler{
		entity:      entity,
		columnNames: make(map[string]string, len(fields)),
		encryptor:   conn.Encryptor(),
	}

	//get marshalled values of entity fields
	marshalledValues, err := entity.Marshaller().Marshal()
	if err != nil {
		return colHdlr, newInvalidEntityError(fmt.Sprintf("failed to marshal values of entity '%s': %s", entity, err.Error()))
	}

	//add columns to column handler instance
	for _, field := range fields {
		col := &column{
			name:     strcase.ToSnake(field.Name()),
			readOnly: hasTag(field, dbTagReadOnly),
			notNull:  hasTag(field, dbTagNotNull),
			encrypt:  hasTag(field, dbTagEncrypt),
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
	var invalidFields []string
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
				return fmt.Errorf("field '%s' of entity '%s' has type '%s' - this type is not supported yet",
					col.field.Name(), ch.entity, col.field.Kind())
			}
		}
	}
	if len(invalidFields) > 0 {
		return newInvalidEntityError("the fields '%s' of entity '%s' are tagged with '%s' and cannot be undefined",
			strings.Join(invalidFields, "', '"), ch.entity, dbTagNotNull)
	}
	return nil
}

func (ch *ColumnHandler) ColumnName(field string) (string, error) {
	if colName, ok := ch.columnNames[field]; ok {
		return colName, nil
	}
	return "", fmt.Errorf("entity '%s' has no field '%s': cannot resolve column name", ch.entity, field)
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
	var result []interface{}
	for _, col := range ch.columns {
		if onlyWriteable && col.readOnly {
			continue
		}
		result = append(result, col.value)
	}
	return result
}

func (ch *ColumnHandler) columnValuesCsvRenderer(onlyWriteable, placeholder bool) (string, error) {
	var err error
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
			value, err := ch.serializeValue(col)
			if err != nil {
				break
			}
			buffer.WriteString(value)
		}

	}
	return buffer.String(), err
}

func (ch *ColumnHandler) ColumnValuesCsv(onlyWriteable bool) (string, error) {
	return ch.columnValuesCsvRenderer(onlyWriteable, false)
}

func (ch *ColumnHandler) ColumnValuesPlaceholderCsv(onlyWriteable bool) (string, error) {
	return ch.columnValuesCsvRenderer(onlyWriteable, true)
}

func (ch *ColumnHandler) columnEntriesCsvRenderer(onlyWriteable, placeholder bool) (string, int, error) {
	var err error
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
			value, err := ch.serializeValue(col)
			if err != nil {
				break
			}
			buffer.WriteString(fmt.Sprintf("%s=%s", col.name, value))
		}
	}
	return buffer.String(), placeholderIdx, err
}

func (ch *ColumnHandler) ColumnEntriesCsv(onlyWriteable bool) (string, int, error) {
	return ch.columnEntriesCsvRenderer(onlyWriteable, false)
}

func (ch *ColumnHandler) ColumnEntriesPlaceholderCsv(onlyWriteable bool) (string, int, error) {
	return ch.columnEntriesCsvRenderer(onlyWriteable, true)
}

func (ch *ColumnHandler) serializeValue(col *column) (string, error) {
	var value string
	switch reflect.ValueOf(col.value).Kind() {
	case reflect.Bool:
		value = fmt.Sprintf("%t", col.value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value = fmt.Sprintf("%d", col.value)
	case reflect.Float32, reflect.Float64:
		value = fmt.Sprintf("%f", col.value)
	default:
		value = fmt.Sprintf("'%v'", col.value)
	}
	if col.encrypt {
		return ch.encryptor.Encrypt(value)
	}
	return value, nil
}

func (ch *ColumnHandler) Unmarshal(row DataRow, entity DatabaseEntity) error {
	var colVals []interface{}
	for _, col := range ch.columns {
		colVals = append(colVals, &col.value)
	}
	if err := row.Scan(colVals...); err != nil {
		return err
	}
	entityData := make(map[string]interface{}, len(ch.columns))
	for _, col := range ch.columns {
		if col.encrypt {
			decValue, err := ch.encryptor.Decrypt(fmt.Sprintf("%s", col.value))
			if err != nil {
				return err
			}
			entityData[col.field.Name()] = decValue
		} else {
			entityData[col.field.Name()] = col.value
		}
	}
	return entity.Marshaller().Unmarshal(entityData)
}
