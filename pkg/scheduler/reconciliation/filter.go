package reconciliation

import (
	"fmt"
	"strings"

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

type WithRuntimeIDs struct {
	RuntimeIDs []string
}

func (wc *WithRuntimeIDs) FilterByQuery(q *db.Select) error {
	runtimeIDsLen := len(wc.RuntimeIDs)
	if runtimeIDsLen < 1 {
		return nil
	}

	var values string
	for i := range wc.RuntimeIDs {
		if i == runtimeIDsLen-1 {
			values = fmt.Sprintf("%s$%d", values, i+1)
			break
		}
		values = fmt.Sprintf("%s$%d,", values, i+1)
	}

	q.WhereIn("RuntimeID", values, strings.Join(wc.RuntimeIDs, ","))
	return nil
}

func (wc *WithRuntimeIDs) FilterByInstance(i *model.ReconciliationEntity) *model.ReconciliationEntity {
	runtimeIDsLen := len(wc.RuntimeIDs)
	if runtimeIDsLen < 1 {
		return nil
	}

	for _, runtimeID := range wc.RuntimeIDs {
		if i.RuntimeID == runtimeID {
			return i
		}
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
