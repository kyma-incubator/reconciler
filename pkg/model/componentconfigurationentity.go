package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblComponentConfiguration string = "component_configurations"

type ComponentConfigurationEntity struct {
	ID        int64  `db:"readOnly" db:"notNull"`
	Cluster   string `db:"notNull"`
	Component string `db:"notNull"`
	Key       string `db:"notNull"`
	Value     string `db:"notNull"`
	Secret    bool   `db:"notNull"`
	Created   time.Time
}

func (c *ComponentConfigurationEntity) String() string {
	return fmt.Sprintf("ComponentConfigurationEntity [Cluster=%s,Component=%s,Key=%s,Value=%s,Secret=%t]",
		c.Cluster, c.Component, c.Key, c.Value, c.Secret)
}

func (c *ComponentConfigurationEntity) New() db.DatabaseEntity {
	return &ComponentConfigurationEntity{}
}

func (c *ComponentConfigurationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *ComponentConfigurationEntity) Table() string {
	return tblComponentConfiguration
}

func (c *ComponentConfigurationEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ComponentConfigurationEntity)
	if ok {
		return c.Cluster == otherClProp.Cluster && c.Component == otherClProp.Component && c.Key == otherClProp.Key && c.Value == otherClProp.Value && c.Secret == otherClProp.Secret
	}
	return false
}
