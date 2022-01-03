package operation

import (
	"bytes"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type Filter interface {
	FilterByQuery(q *db.Select) error
	FilterByInstance(i *model.OperationEntity) *model.OperationEntity //return nil to ignore instance in result
}

type FilterMixer struct {
	Filters []Filter
}

func (fm *FilterMixer) FilterByQuery(q *db.Select) error {
	for i := range fm.Filters {
		if err := fm.Filters[i].FilterByQuery(q); err != nil {
			return err
		}
	}
	return nil
}

func (fm *FilterMixer) FilterByInstance(re *model.OperationEntity) *model.OperationEntity {
	entity := re
	for i := range fm.Filters {
		entity = fm.Filters[i].FilterByInstance(entity)
		if entity == nil {
			break
		}
	}
	return entity
}

type WithSchedulingID struct {
	SchedulingID string
}

func (ws *WithSchedulingID) FilterByQuery(q *db.Select) error {
	q.Where(map[string]interface{}{
		"SchedulingID": ws.SchedulingID,
	})
	return nil
}

func (ws *WithSchedulingID) FilterByInstance(i *model.OperationEntity) *model.OperationEntity {
	if i.SchedulingID == ws.SchedulingID {
		return i
	}
	return nil
}

type WithStates struct {
	States []model.OperationState
}

func (ws *WithStates) FilterByQuery(q *db.Select) error {
	var args []interface{}
	var buffer bytes.Buffer

	for idx, state := range ws.States {
		args = append(args, state)
		if buffer.Len() > 0 {
			buffer.WriteRune(',')
		}
		buffer.WriteString(fmt.Sprintf("$%d", idx+2))
	}

	q.WhereIn("State", buffer.String(), args...)
	return nil
}

func (ws *WithStates) FilterByInstance(re *model.OperationEntity) *model.OperationEntity {
	if len(ws.States) < 1 {
		return nil
	}

	for i := range ws.States {
		if ws.States[i] == re.State {
			return re
		}
	}
	return nil
}

func toInterfaceSlice(args []string) []interface{} {
	argsLen := len(args)
	result := make([]interface{}, argsLen)
	for i := 0; i < argsLen; i++ {
		result[i] = args[i]
	}
	return result
}

func columnName(q *db.Select, name string) (string, error) {
	statusColHandler, err := db.NewColumnHandler(&model.OperationEntity{}, q.Conn, q.Logger)
	if err != nil {
		return "", err
	}
	return statusColHandler.ColumnName(name)
}
