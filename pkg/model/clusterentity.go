package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
)

const tblCluster string = "inventory_clusters"

type ClusterEntity struct {
	Version    int64             `db:"readOnly"`
	RuntimeID  string            `db:"notNull"`
	Runtime    *keb.RuntimeInput `db:"notNull"`
	Metadata   *keb.Metadata     `db:"notNull"`
	Kubeconfig string            `db:"notNull,encrypt"`
	Contract   int64             `db:"notNull"`
	Deleted    bool              `db:"notNull"`
	Created    time.Time         `db:"readOnly"`
}

func (c *ClusterEntity) String() string {
	return fmt.Sprintf("ClusterEntity [RuntimeID=%s,Version=%d]", c.RuntimeID, c.Version)
}

func (c *ClusterEntity) New() db.DatabaseEntity {
	return &ClusterEntity{}
}

func (c *ClusterEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)
	marshaller.AddUnmarshaller("Runtime", func(value interface{}) (interface{}, error) {
		var runtimeInput *keb.RuntimeInput
		err := json.Unmarshal([]byte(value.(string)), &runtimeInput)
		return runtimeInput, err
	})
	marshaller.AddUnmarshaller("Metadata", func(value interface{}) (interface{}, error) {
		var metadata *keb.Metadata
		err := json.Unmarshal([]byte(value.(string)), &metadata)
		return metadata, err
	})

	marshaller.AddMarshaller("Runtime", convertInterfaceToJSONString)
	marshaller.AddMarshaller("Metadata", convertInterfaceToJSONString)
	return marshaller
}

func (c *ClusterEntity) Table() string {
	return tblCluster
}

func (c *ClusterEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ClusterEntity)
	if ok {
		return c.RuntimeID == otherClProp.RuntimeID &&
			reflect.DeepEqual(c.Runtime, otherClProp.Runtime) &&
			reflect.DeepEqual(c.Metadata, otherClProp.Metadata) &&
			c.Contract == otherClProp.Contract
	}
	return false
}
