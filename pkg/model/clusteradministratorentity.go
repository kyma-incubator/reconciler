package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblClusterAdministrators string = "cluster_administrators"

type ClusterAdministratorEntity struct {
	ID      int64  `db:"readOnly" db:"notNull"`
	Cluster string `db:"notNull"`
	UserId  string `db:"notNull"`
	Created time.Time
}

func (c *ClusterAdministratorEntity) String() string {

	return fmt.Sprintf("ClusterAdministratorEntity [Cluster=%s,UserId=%s]",
		c.Cluster, c.UserId)
}

func (c *ClusterAdministratorEntity) New() db.DatabaseEntity {
	return &ClusterAdministratorEntity{}
}

func (c *ClusterAdministratorEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ClusterAdministratorEntity) Table() string {
	return tblClusterAdministrators
}

func (c *ClusterAdministratorEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ClusterAdministratorEntity)
	if ok {
		return c.Cluster == otherClProp.Cluster && c.UserId == otherClProp.UserId
	}
	return false
}
