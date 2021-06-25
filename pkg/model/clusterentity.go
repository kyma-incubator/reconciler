package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblCluster string = "clusters"

type ClusterEntity struct {
	ID                 int64         `db:"readOnly" db:"notNull"`
	Cluster            string        `db:"notNull"`
	Status             ClusterStatus `db:"notNull"`
	RuntimeName        string
	RuntimeDescription string
	KymaVersion        string
	KymaProfile        string
	GlobalAccountID    string
	SubAccountID       string
	ServiceID          string
	ServicePlanID      string
	ShootName          string
	InstanceID         string
	Created            time.Time
}

func (c *ClusterEntity) String() string {
	return fmt.Sprintf("ClusterEntity [Cluster=%s,Status=%s]",
		c.Cluster, c.Status)
}

func (c *ClusterEntity) New() db.DatabaseEntity {
	return &ClusterEntity{}
}

func (c *ClusterEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Status", func(value interface{}) (interface{}, error) {
		return NewClusterStatus(value.(string))
	})
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ClusterEntity) Table() string {
	return tblCluster
}

func (c *ClusterEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ClusterEntity)
	if ok {
		return c.Cluster == otherClProp.Cluster
	}
	return false
}
