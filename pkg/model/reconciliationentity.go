package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblReconciliation string = "scheduler_reconciliations"

type ReconciliationEntity struct {
	Lock                string    `db:"notNull"`
	RuntimeID           string    `db:"notNull"`
	ClusterConfig       int64     `db:"notNull"`
	ClusterConfigStatus int64     `db:"notNull"`
	Finished            bool      `db:"notNull"`
	SchedulingID        string    `db:"notNull"`
	Created             time.Time `db:"readOnly"`
	Updated             time.Time `db:""`
	Status              Status    `db:"notNull"`
}

func (r *ReconciliationEntity) String() string {
	return fmt.Sprintf("ReconciliationEntity [Cluster=%s,ClusterConfigVersion=%d,SchedulingID=%s]",
		r.RuntimeID, r.ClusterConfig, r.SchedulingID)
}

func (*ReconciliationEntity) New() db.DatabaseEntity {
	return &ReconciliationEntity{}
}

func (r *ReconciliationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&r)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	marshaller.AddUnmarshaller("Updated", convertTimestampToTime)
	marshaller.AddUnmarshaller("Status", convertStringToStatus)
	return marshaller
}

func (*ReconciliationEntity) Table() string {
	return tblReconciliation
}

func (r *ReconciliationEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherOpProp, ok := other.(*ReconciliationEntity)
	if !ok {
		return false
	}
	return r.RuntimeID == otherOpProp.RuntimeID && r.SchedulingID == otherOpProp.SchedulingID
}
