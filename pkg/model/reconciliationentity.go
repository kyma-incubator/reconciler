package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblReconciliation string = "scheduler_reconciliations"

type ReconciliationEntity struct {
	Lock                string    `db:"notNull"`
	Cluster             string    `db:"notNull"`
	ClusterConfig       int64     `db:"notNull"`
	ClusterConfigStatus int64     `db:""`
	SchedulingID        string    `db:"notNull"`
	Created             time.Time `db:"readOnly"`
	Updated             time.Time `db:""`
}

func (r *ReconciliationEntity) String() string {
	return fmt.Sprintf("ReconciliationEntity [Cluster=%s,ClusterConfigVersion=%d,SchedulingID=%s]",
		r.Cluster, r.ClusterConfig, r.SchedulingID)
}

func (*ReconciliationEntity) New() db.DatabaseEntity {
	return &ReconciliationEntity{}
}

func (r *ReconciliationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&r)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	marshaller.AddUnmarshaller("Updated", convertTimestampToTime)
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
	return r.Cluster == otherOpProp.Cluster && r.SchedulingID == otherOpProp.SchedulingID
}
