package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblConfiguration string = "inventory_cluster_configs"

type ClusterConfigurationEntity struct {
	Version        int64  `db:"readOnly"`
	Cluster        string `db:"notNull"`
	ClusterVersion int64  `db:"notNull"`
	KymaVersion    string `db:"notNull"`
	KymaProfile    string `db:"notNull"`
	Components     string `db:"notNull"`
	Administrators string
	Created        time.Time `db:"readOnly"`
}

func (c *ClusterConfigurationEntity) String() string {
	return fmt.Sprintf("ClusterConfigurationEntity [Version=%d,Cluster=%s,ClusterVersion=%d]",
		c.Version, c.Cluster, c.ClusterVersion)
}

func (c *ClusterConfigurationEntity) New() db.DatabaseEntity {
	return &ClusterConfigurationEntity{}
}

func (c *ClusterConfigurationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ClusterConfigurationEntity) Table() string {
	return tblConfiguration
}

func (c *ClusterConfigurationEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ClusterConfigurationEntity)
	if ok {
		return c.Version == otherClProp.Version &&
			c.Cluster == otherClProp.Cluster &&
			c.ClusterVersion == otherClProp.ClusterVersion
	}
	return false
}
