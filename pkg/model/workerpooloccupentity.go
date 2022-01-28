package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblOccupancy string = "worker_pool_occupancy"

type WorkerPoolOccupancyEntity struct {
	WorkerPoolID       string    `db:"notNull"`
	Component          string    `db:"notNull"`
	RunningWorkers     int64     `db:""`
	WorkerPoolCapacity int64     `db:"notNull"`
	Created            time.Time `db:"readOnly"`
}

func (o *WorkerPoolOccupancyEntity) String() string {
	return fmt.Sprintf("WorkerPoolOccupancyEntity [Component=%s,RunningWorkers=%d,WorkerPoolCapacity=%d]",
		o.Component, o.RunningWorkers, o.WorkerPoolCapacity)
}

func (o *WorkerPoolOccupancyEntity) New() db.DatabaseEntity {
	return &WorkerPoolOccupancyEntity{}
}

func (o *WorkerPoolOccupancyEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&o)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (o *WorkerPoolOccupancyEntity) Table() string {
	return tblOccupancy
}

func (o *WorkerPoolOccupancyEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherOcProp, ok := other.(*WorkerPoolOccupancyEntity)
	if ok {
		return o.WorkerPoolID == otherOcProp.WorkerPoolID
	}
	return false
}
