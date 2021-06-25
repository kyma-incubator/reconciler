package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblComponent string = "components"

type ComponentEntity struct {
	ID        int64  `db:"readOnly" db:"notNull"`
	Cluster   string `db:"notNull"`
	Component string `db:"notNull"`
	Namespace string `db:"notNull"`
	Created   time.Time
}

func (c *ComponentEntity) String() string {
	return fmt.Sprintf("ComponentEntity [Cluster=%s,Component=%s,Namespace=%s]",
		c.Cluster, c.Component, c.Namespace)
}

func (c *ComponentEntity) New() db.DatabaseEntity {
	return &ComponentEntity{}
}

func (c *ComponentEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
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
		return c.Cluster == otherClProp.Cluster && c.Component == otherClProp.Component && c.Namespace == otherClProp.Namespace
	}
	return false
}
