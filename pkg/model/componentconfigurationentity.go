package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblComponentConfiguration string = "component_configurations"

type ComponentConfigurationEntity struct {
	ID          string `db:"notNull"`
	ComponentID string `db:"notNull"`
	Key         string `db:"notNull"`
	Value       string `db:"notNull"`
	Secret      bool   `db:"notNull"`
	Created     time.Time
}

func (c *ComponentConfigurationEntity) String() string {
	return fmt.Sprintf("ComponentConfigurationEntity [ComponentID=%s,Key=%s,Value=%s,Secret=%t]",
		c.ComponentID, c.Key, c.Value, c.Secret)
}

func (c *ComponentConfigurationEntity) New() db.DatabaseEntity {
	return &ComponentConfigurationEntity{}
}

func (c *ComponentConfigurationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("ID", func(value interface{}) (interface{}, error) {
		return string(value.([]uint8)), nil
	})
	marshaller.AddUnmarshaller("ComponentID", func(value interface{}) (interface{}, error) {
		return string(value.([]uint8)), nil
	})
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
		return c.ComponentID == otherClProp.ComponentID && c.Key == otherClProp.Key && c.Value == otherClProp.Value && c.Secret == otherClProp.Secret
	}
	return false
}
