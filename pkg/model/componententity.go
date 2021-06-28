package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblComponent string = "components"

type ComponentEntity struct {
	ID              string `db:"notNull"`
	ConfigurationID string `db:"notNull"`
	Component       string `db:"notNull"`
	Namespace       string `db:"notNull"`
	Created         time.Time
}

func (c *ComponentEntity) String() string {
	return fmt.Sprintf("ComponentEntity [ConfigurationID=%s,Component=%s,Namespace=%s]",
		c.ConfigurationID, c.Component, c.Namespace)
}

func (c *ComponentEntity) New() db.DatabaseEntity {
	return &ComponentEntity{}
}

func (c *ComponentEntity) Marshaller() *db.EntityMarshaller {
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

func (c *ComponentEntity) Table() string {
	return tblComponent
}

func (c *ComponentEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ComponentEntity)
	if ok {
		return c.ConfigurationID == otherClProp.ConfigurationID && c.Component == otherClProp.Component && c.Namespace == otherClProp.Namespace
	}
	return false
}
