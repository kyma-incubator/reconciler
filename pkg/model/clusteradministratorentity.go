package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblClusterAdministrators string = "cluster_administrators"

type ClusterAdministratorEntity struct {
	ID              string `db:"notNull"`
	ConfigurationID string `db:"notNull"`
	UserId          string `db:"notNull"`
	Created         time.Time
}

func (c *ClusterAdministratorEntity) String() string {

	return fmt.Sprintf("ClusterAdministratorEntity [ConfigurationID=%s,UserId=%s]",
		c.ConfigurationID, c.UserId)
}

func (c *ClusterAdministratorEntity) New() db.DatabaseEntity {
	return &ClusterAdministratorEntity{}
}

func (c *ClusterAdministratorEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("ID", func(value interface{}) (interface{}, error) {
		return string(value.([]uint8)), nil
	})
	marshaller.AddUnmarshaller("ConfigurationID", func(value interface{}) (interface{}, error) {
		return string(value.([]uint8)), nil
	})
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
		return c.ConfigurationID == otherClProp.ConfigurationID && c.UserId == otherClProp.UserId
	}
	return false
}
