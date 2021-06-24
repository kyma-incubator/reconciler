package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblCluster string = "clusters"

type ClusterEntity struct {
	ID             string        `db:"notNull"`
	Cluster        string        `db:"notNull"`
	Status         ClusterStatus `db:"notNull"`
	ComponentsList string        `db:"notNull"`
	Created        time.Time     `db:"readOnly"`
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
	marshaller.AddUnmarshaller("ClusterState", func(value interface{}) (interface{}, error) {
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
		return c.Cluster == otherClProp.Cluster && c.Status == otherClProp.Status
	}
	return false
}
