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

type WithCluster struct {
	Cluster string
}

func (wc *WithCluster) FilterByQuery(q *db.Select) error {
	q.Where(map[string]interface{}{
		"Cluster": wc.Cluster,
	})
	return nil
}

func (wc *WithCluster) FilterByInstance(i *model.ReconciliationEntity) *model.ReconciliationEntity {
	if i.Cluster == wc.Cluster {
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
