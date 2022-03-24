package model

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"time"
)

const tblClusterCleanup string = "v_inventory_cluster_cleanup"

type ClusterCleanupEntity struct {
	StatusID  int64     `db:"notNull"`
	RuntimeID string    `db:"notNull"`
	ClusterID int64     `db:"notNull"`
	ConfigID  int64     `db:"notNull"`
	Status    string    `db:"notNull"`
	Created   time.Time `db:"readOnly"`
}

func (cce *ClusterCleanupEntity) String() string {
	return fmt.Sprintf("ClusterCleanupEntity [Status=%s,Created=%d,RuntimeID=%s]", cce.Status, cce.Created.Second(), cce.RuntimeID)
}

func (cce *ClusterCleanupEntity) New() db.DatabaseEntity {
	return &ClusterCleanupEntity{}
}

func (cce *ClusterCleanupEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&cce)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (cce *ClusterCleanupEntity) Table() string {
	return tblClusterCleanup
}

func (cce *ClusterCleanupEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherOcProp, ok := other.(*ClusterCleanupEntity)
	if ok {
		return cce.StatusID == otherOcProp.StatusID
	}
	return false
}

const tblStatusCleanup string = "v_inventory_status_cleanup"

type StatusCleanupEntity struct {
	StatusID  int64     `db:"notNull"`
	RuntimeID string    `db:"notNull"`
	ClusterID int64     `db:"notNull"`
	ConfigID  int64     `db:"notNull"`
	Status    string    `db:"notNull"`
	Created   time.Time `db:"readOnly"`
}

func (sce *StatusCleanupEntity) String() string {
	return fmt.Sprintf("StatusCleanupEntity [Status=%s,Created=%d,RuntimeID=%s]", sce.Status, sce.Created.Second(), sce.RuntimeID)
}

func (sce *StatusCleanupEntity) New() db.DatabaseEntity {
	return &StatusCleanupEntity{}
}

func (sce *StatusCleanupEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&sce)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (sce *StatusCleanupEntity) Table() string {
	return tblStatusCleanup
}

func (sce *StatusCleanupEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherOcProp, ok := other.(*StatusCleanupEntity)
	if ok {
		return sce.StatusID == otherOcProp.StatusID
	}
	return false
}
