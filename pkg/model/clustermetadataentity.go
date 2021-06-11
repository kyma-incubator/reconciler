package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblClusterMetadata string = "cluster_metadata"

type ClusterMetadataEntity struct {
	ID      string    `db:"notNull"`
	Cluster string    `db:"notNull"`
	Key     string    `db:"notNull"`
	Value   string    `db:"notNull"`
	Created time.Time `db:"readOnly"`
}

func (c *ClusterMetadataEntity) String() string {
	return fmt.Sprintf("ClusterMetadataEntity [Cluster=%s,Key=%s,Value=%s]",
		c.Cluster, c.Key, c.Value)
}

func (c *ClusterMetadataEntity) New() db.DatabaseEntity {
	return &ClusterMetadataEntity{}
}

func (c *ClusterMetadataEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ClusterMetadataEntity) Table() string {
	return tblClusterMetadata
}

func (c *ClusterMetadataEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClMeta, ok := other.(*ClusterMetadataEntity)
	if ok {
		return c.Cluster == otherClMeta.Cluster && c.Key == otherClMeta.Key
	}
	return false
}
