package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblConfiguration string = "configurations"

type ConfigurationEntity struct {
	ID             int64 `db:"readOnly" db:"notNull"`
	ClusterID      int64 `db:"notNull"`
	KymaVersion    string
	KymaProfile    string
	Components     string
	Administrators string
	Created        time.Time
}

func (c *ConfigurationEntity) String() string {
	return fmt.Sprintf("ConfigurationEntity [ID=%s,ClusterID=%s,KymaVersion=%s,KymaProfile=%s,Components=%s,Administrators=%s]",
		c.ID, c.ClusterID, c.KymaVersion, c.KymaProfile, c.Components, c.Administrators)
}

func (c *ConfigurationEntity) New() db.DatabaseEntity {
	return &ConfigurationEntity{}
}

func (c *ConfigurationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
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
		return c.ID == otherClProp.ID && c.ClusterID == otherClProp.ClusterID && c.KymaVersion == otherClProp.KymaVersion && c.KymaProfile == otherClProp.KymaProfile && c.Administrators == otherClProp.Administrators
	}
	return false
}
