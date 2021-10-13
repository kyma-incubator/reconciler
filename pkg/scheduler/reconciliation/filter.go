package reconciliation

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type WithSchedulingID struct {
	SchedulingID string
}

func (ws *WithSchedulingID) FilterByQuery(q *db.Select) error {
	q.Where(map[string]interface{}{
		"SchedulingID": ws.SchedulingID,
	})
	return nil
}

func (ws *WithSchedulingID) FilterByInstance(i *model.ReconciliationEntity) *model.ReconciliationEntity {
	if i.SchedulingID == ws.SchedulingID {
		return i
	}
	return nil
}

type WithRuntimeID struct {
	RuntimeID string
}

func (wc *WithRuntimeID) FilterByQuery(q *db.Select) error {
	q.Where(map[string]interface{}{
		"RuntimeID": wc.RuntimeID,
	})
	return nil
}

func (wc *WithRuntimeID) FilterByInstance(i *model.ReconciliationEntity) *model.ReconciliationEntity {
	if i.RuntimeID == wc.RuntimeID {
		return i
	}
	return nil
}

type CurrentlyReconciling struct {
}

func (cr *CurrentlyReconciling) FilterByQuery(q *db.Select) error {
	q.Where(map[string]interface{}{
		"Finished": false,
	})
	return nil
}

func (cr *CurrentlyReconciling) FilterByInstance(i *model.ReconciliationEntity) *model.ReconciliationEntity {
	if !i.Finished {
		return i
	}
	return nil
}
