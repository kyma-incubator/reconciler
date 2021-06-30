package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblCluster string = "inventory_clusters"

type ClusterEntity struct {
	Version            int64     `db:"readOnly"`
	Cluster            string    `db:"notNull"`
	RuntimeName        string    `db:"notNull"`
	RuntimeDescription string    `db:"notNull"`
	Metadata           string    `db:"notNull"`
	Created            time.Time `db:"readOnly"`
}

func (c *ClusterEntity) String() string {
	return fmt.Sprintf("ClusterEntity [Cluster=%s,Version=%d]",
		c.Cluster, c.Version)
}

func (c *ClusterEntity) New() db.DatabaseEntity {
	return &ClusterEntity{}
}

func (c *ClusterEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
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
