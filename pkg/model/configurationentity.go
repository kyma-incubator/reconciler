package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblConfiguration string = "configurations"

type ConfigurationEntity struct {
	ID          string `db:"notNull"`
	ClusterID   string `db:"notNull"`
	KymaVersion string
	KymaProfile string
	Created     time.Time
}

func (c *ConfigurationEntity) String() string {
	return fmt.Sprintf("ConfigurationEntity [ID=%s,ClusterID=%s,KymaVersion=%s,KymaProfile=%s]",
		c.ID, c.ClusterID, c.KymaVersion, c.KymaProfile)
}

func (c *ConfigurationEntity) New() db.DatabaseEntity {
	return &ConfigurationEntity{}
}

func (c *ConfigurationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("ID", func(value interface{}) (interface{}, error) {
		return string(value.([]uint8)), nil
	})
	marshaller.AddUnmarshaller("ClusterID", func(value interface{}) (interface{}, error) {
		return string(value.([]uint8)), nil
	})
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ConfigurationEntity) Table() string {
	return tblConfiguration
}

func (c *ConfigurationEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ConfigurationEntity)
	if ok {
		return c.ID == otherClProp.ID && c.ClusterID == otherClProp.ClusterID && c.KymaVersion == otherClProp.KymaVersion && c.KymaProfile == otherClProp.KymaProfile
	}
	return false
}
