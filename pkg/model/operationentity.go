package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblOperation string = "scheduler_operations"

type OperationEntity struct {
	SchedulingID  string         `db:"notNull"`
	CorrelationID string         `db:"notNull"`
	ConfigVersion int64          `db:"notNull"`
	Component     string         `db:"notNull"`
	State         OperationState `db:"notNull"`
	Reason        string         `db:""`
	Created       time.Time      `db:"readOnly"`
	Updated       time.Time      `db:""`
}

func (o *OperationEntity) String() string {
	return fmt.Sprintf("OperationEntity [SchedulingID=%s,CorrelationID=%s,ConfigVersion=%d,Component=%s]",
		o.SchedulingID, o.CorrelationID, o.ConfigVersion, o.Component)
}

func (*OperationEntity) New() db.DatabaseEntity {
	return &OperationEntity{}
}

func (o *OperationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&o)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	marshaller.AddUnmarshaller("Updated", convertTimestampToTime)
	return marshaller
}

func (*OperationEntity) Table() string {
	return tblOperation
}

func (o *OperationEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherOpProp, ok := other.(*OperationEntity)
	if !ok {
		return false
	}
	return o.SchedulingID == otherOpProp.SchedulingID &&
		o.CorrelationID == otherOpProp.CorrelationID &&
		o.Component == otherOpProp.Component
}
