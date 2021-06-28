package model

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

const tblStatuses string = "statuses"

type StatusEntity struct {
	ID              int64         `db:"readOnly" db:"notNull"`
	ConfigurationID int64         `db:"notNull"`
	Status          ClusterStatus `db:"notNull"`
	Created         time.Time
}

func (c *StatusEntity) String() string {

	return fmt.Sprintf("StatusEntity [ConfigurationID=%s,Status=%s]",
		c.ConfigurationID, c.Status)
}

func (c *StatusEntity) New() db.DatabaseEntity {
	return &StatusEntity{}
}

func (c *StatusEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Status", func(value interface{}) (interface{}, error) {
		return NewClusterStatus(value.(string))
	})
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	return marshaller
}

func (c *StatusEntity) Table() string {
	return tblStatuses
}

func (c *StatusEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*StatusEntity)
	if ok {
		return c.ConfigurationID == otherClProp.ConfigurationID && c.Status == otherClProp.Status
	}
	return false
}
