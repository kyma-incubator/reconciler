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

	argsOffset := q.NextPlaceholderCount()
	for i := range ws.States {
		args = append(args, ws.States[i])
		if buffer.Len() > 0 {
			buffer.WriteRune(',')
		}
		buffer.WriteString(fmt.Sprintf("$%d", argsOffset+i))
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

type WithCorrelationID struct {
	CorrelationID string
}

func (ws *WithCorrelationID) FilterByQuery(q *db.Select) error {
	q.Where(map[string]interface{}{
		"CorrelationID": ws.CorrelationID,
	})
	return nil
}

func (ws *WithCorrelationID) FilterByInstance(i *model.OperationEntity) *model.OperationEntity {
	if i.CorrelationID == ws.CorrelationID {
		return i
	}
	return nil
}

type WithComponentName struct {
	Component string
}

func (wc *WithComponentName) FilterByQuery(q *db.Select) error {
	q.Where(map[string]interface{}{
		"Component": wc.Component,
	})
	return nil
}

func (wc *WithComponentName) FilterByInstance(i *model.OperationEntity) *model.OperationEntity {
	if i.Component == wc.Component {
		return i
	}
	return nil
}

type Limit struct {
	Count       int
	actualCount int
}

func (l *Limit) FilterByQuery(q *db.Select) error {
	q.OrderBy(map[string]string{"Created": "DESC"}).Limit(l.Count)
	return nil
}

func (l *Limit) FilterByInstance(re *model.OperationEntity) *model.OperationEntity {
	if l.actualCount < l.Count {
		l.actualCount++
		return re
	}
	return nil
}

type LimitByLastUpdate struct {
	Count       int
	actualCount int
}

func (l *LimitByLastUpdate) FilterByQuery(q *db.Select) error {
	q.OrderBy(map[string]string{"Updated": "DESC"}).Limit(l.Count)
	return nil
}

func (l *LimitByLastUpdate) FilterByInstance(re *model.OperationEntity) *model.OperationEntity {
	if l.actualCount < l.Count {
		l.actualCount++
		return re
	}
	return nil
}
