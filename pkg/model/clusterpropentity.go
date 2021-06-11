package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblClusterProperties string = "cluster_metadata"

type ClusterPropertyEntity struct {
	ID      string    `db:"notNull"`
	Cluster string    `db:"notNull"`
	Key     string    `db:"notNull"`
	Value   string    `db:"notNull"`
	Created time.Time `db:"readOnly"`
}

func (c *ClusterPropertyEntity) String() string {
	return fmt.Sprintf("ClusterPropertyEntity [Cluster=%s,Key=%s,Value=%s]",
		c.Cluster, c.Key, c.Value)
}

func (c *ClusterPropertyEntity) New() db.DatabaseEntity {
	return &ClusterPropertyEntity{}
}

func (c *ClusterPropertyEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ClusterPropertyEntity) Table() string {
	return tblClusterProperties
}

func (c *ClusterPropertyEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ClusterPropertyEntity)
	if ok {
		return c.Cluster == otherClProp.Cluster && c.Key == otherClProp.Key
	}
	return false
}
